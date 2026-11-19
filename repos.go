package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// repoLane is one repository's weekly commit counts over the window.
type repoLane struct {
	name     string    // display name: bare repo name for the user's own repos, owner/name otherwise
	pushedAt time.Time // tie-break for ranking
	lastAt   time.Time // latest counted commit (author date); zero if none
	weeks    []int     // commits per window week, oldest first
	total    int
}

// laneBatch is how many repos' commits are fetched concurrently.
const laneBatch = 8

// fetchLanes picks the cfg.nRepos repos with the most recent commits in the
// window (the user's own commits, or anyone's with --all-authors) and returns
// their weekly commit counts, most recently active first.
//
// GitHub has no "repos I last committed to" listing, so we walk the repos the
// user can see in pushed_at order and list each one's commits in the window. A
// repo's last commit can't be later than its last push, so once the next
// candidate's push is no newer than our nth-best repo's last commit, nothing
// further down the list can rank higher and we stop.
//
// (The stats/contributors endpoint would give weekly counts in one request per
// repo, but it answers 202 until GitHub has computed them in the background,
// which can take minutes per repo.)
func fetchLanes(cfg config, weeks []time.Time, sp *spinner) ([]repoLane, error) {
	c, err := newGHClient()
	if err != nil {
		return nil, err
	}

	listURL := fmt.Sprintf("https://api.github.com/users/%s/repos?sort=pushed&per_page=50", cfg.username)
	if c.token != "" {
		var me struct {
			Login string `json:"login"`
		}
		if err := c.getJSON("https://api.github.com/user", &me); err != nil {
			return nil, err
		}
		if strings.EqualFold(me.Login, cfg.username) {
			// Your own repos plus collaborator and org repos, private ones included.
			listURL = "https://api.github.com/user/repos?sort=pushed&per_page=50"
		}
	}
	lister := &repoLister{c: c, next: listURL, since: weeks[0]}

	var lanes []repoLane
	checked, commits := 0, 0
	for {
		var batch []ghRepo
		for len(batch) < laneBatch {
			r, ok, err := lister.Next()
			if err != nil {
				return nil, err
			}
			if !ok {
				break
			}
			batch = append(batch, r)
		}
		if len(batch) == 0 {
			break
		}
		if len(lanes) >= cfg.nRepos {
			sortLanes(lanes)
			if !batch[0].PushedAt.After(lanes[cfg.nRepos-1].lastAt) {
				break // candidates are in push order, so the rest of the batch can't rank either
			}
		}

		sp.Status("reading commits · %d repos checked · %d commits", checked, commits)
		results := make([]repoLane, len(batch))
		errs := make([]error, len(batch))
		var wg sync.WaitGroup
		for i, r := range batch {
			wg.Add(1)
			go func() {
				defer wg.Done()
				results[i], errs[i] = c.repoLane(r, cfg, weeks)
			}()
		}
		wg.Wait()

		for i, l := range results {
			if errs[i] != nil {
				sp.Warnf("warning: skipping %s: %v\n", batch[i].FullName, errs[i])
				continue
			}
			checked++
			commits += l.total
			if l.total > 0 {
				lanes = append(lanes, l)
			}
		}
	}

	sortLanes(lanes)
	if len(lanes) > cfg.nRepos {
		lanes = lanes[:cfg.nRepos]
	}
	return lanes, nil
}

// sortLanes orders lanes by latest commit, then by latest push.
func sortLanes(lanes []repoLane) {
	sort.SliceStable(lanes, func(i, j int) bool {
		if !lanes[i].lastAt.Equal(lanes[j].lastAt) {
			return lanes[i].lastAt.After(lanes[j].lastAt)
		}
		return lanes[i].pushedAt.After(lanes[j].pushedAt)
	})
}

// windowWeeks lists the Sunday (UTC midnight) starting each week that overlaps
// [from, to] — the same week columns as the heatmap grid, and the same week
// boundaries GitHub's own stats use.
func windowWeeks(from, to time.Time) []time.Time {
	var weeks []time.Time
	for ws := from.AddDate(0, 0, -int(from.Weekday())); !ws.After(to); ws = ws.AddDate(0, 0, 7) {
		weeks = append(weeks, ws)
	}
	return weeks
}

// weekIndex is the index of the window week containing t (by its UTC date),
// -1 if it's before the window, clamped to the last week if after.
func weekIndex(weeks []time.Time, t time.Time) int {
	d := date(t.UTC().Year(), t.UTC().Month(), t.UTC().Day())
	i := int(d.Sub(weeks[0]).Hours() / 24 / 7)
	return max(-1, min(i, len(weeks)-1))
}

type ghRepo struct {
	Name     string    `json:"name"`
	FullName string    `json:"full_name"`
	PushedAt time.Time `json:"pushed_at"`
	Owner    struct {
		Login string `json:"login"`
	} `json:"owner"`
}

// repoLister pages through a pushed-order repo listing, stopping at the first
// repo last pushed before since (it can't have commits in the window).
type repoLister struct {
	c     *ghClient
	next  string
	buf   []ghRepo
	since time.Time
	done  bool
}

var nextLink = regexp.MustCompile(`<([^>]+)>;\s*rel="next"`)

func (l *repoLister) Next() (ghRepo, bool, error) {
	for len(l.buf) == 0 {
		if l.done || l.next == "" {
			return ghRepo{}, false, nil
		}
		resp, err := l.c.get(l.next)
		if err != nil {
			return ghRepo{}, false, err
		}
		if err := checkStatus(resp, l.next); err != nil {
			resp.Body.Close()
			return ghRepo{}, false, err
		}
		var page []ghRepo
		err = json.NewDecoder(resp.Body).Decode(&page)
		resp.Body.Close()
		if err != nil {
			return ghRepo{}, false, fmt.Errorf("decoding repo list: %w", err)
		}
		l.next = ""
		if m := nextLink.FindStringSubmatch(resp.Header.Get("Link")); m != nil {
			l.next = m[1]
		}
		l.buf = page
	}
	r := l.buf[0]
	l.buf = l.buf[1:]
	if r.PushedAt.Before(l.since) {
		l.done = true
		return ghRepo{}, false, nil
	}
	return r, true, nil
}

type ghCommit struct {
	Commit struct {
		Author struct {
			Date time.Time `json:"date"`
		} `json:"author"`
	} `json:"commit"`
}

// repoLane lists a repo's default-branch commits in the window — the user's
// own, or everyone's with --all-authors — and buckets them by author date into
// window weeks.
func (c *ghClient) repoLane(r ghRepo, cfg config, weeks []time.Time) (repoLane, error) {
	l := repoLane{name: r.FullName, pushedAt: r.PushedAt, weeks: make([]int, len(weeks))}
	if strings.EqualFold(r.Owner.Login, cfg.username) {
		l.name = r.Name
	}

	q := url.Values{"since": {weeks[0].Format(time.RFC3339)}, "per_page": {"100"}}
	if !cfg.allAuthors {
		q.Set("author", cfg.username)
	}
	next := fmt.Sprintf("https://api.github.com/repos/%s/commits?%s", r.FullName, q.Encode())
	for next != "" {
		resp, err := c.get(next)
		if err != nil {
			return repoLane{}, err
		}
		if resp.StatusCode == http.StatusConflict { // empty repository
			resp.Body.Close()
			break
		}
		if err := checkStatus(resp, next); err != nil {
			resp.Body.Close()
			return repoLane{}, err
		}
		var page []ghCommit
		err = json.NewDecoder(resp.Body).Decode(&page)
		resp.Body.Close()
		if err != nil {
			return repoLane{}, fmt.Errorf("decoding commits: %w", err)
		}
		next = ""
		if m := nextLink.FindStringSubmatch(resp.Header.Get("Link")); m != nil {
			next = m[1]
		}

		for _, cm := range page {
			at := cm.Commit.Author.Date
			i := weekIndex(weeks, at)
			if i < 0 {
				continue // authored before the window, committed (e.g. rebased) after
			}
			l.weeks[i]++
			l.total++
			if at.After(l.lastAt) {
				l.lastAt = at
			}
		}
	}
	return l, nil
}

// ghClient talks to the GitHub REST API, authenticated when a token is
// available so private repos are included.
type ghClient struct {
	token string
}

// newGHClient takes a token from GH_TOKEN / GITHUB_TOKEN, else from the gh CLI.
// Without one, only public repos are visible (and the rate limit is low).
func newGHClient() (*ghClient, error) {
	for _, k := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
		if t := os.Getenv(k); t != "" {
			return &ghClient{token: t}, nil
		}
	}
	if path, err := exec.LookPath("gh"); err == nil {
		if out, err := exec.Command(path, "auth", "token").Output(); err == nil {
			return &ghClient{token: strings.TrimSpace(string(out))}, nil
		}
	}
	return &ghClient{}, nil
}

func (c *ghClient) get(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "gh-contribs (https://github.com/Josef-Hlink/gh-contribs)")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting %s: %w", url, err)
	}
	return resp, nil
}

func (c *ghClient) getJSON(url string, v any) error {
	resp, err := c.get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := checkStatus(resp, url); err != nil {
		return err
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

func checkStatus(resp *http.Response, url string) error {
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized:
		return fmt.Errorf("GitHub rejected the token (401) — run `gh auth login` again")
	case http.StatusForbidden, http.StatusTooManyRequests:
		if resp.Header.Get("X-RateLimit-Remaining") == "0" {
			return fmt.Errorf("GitHub API rate limit reached — sign in with `gh auth login` or try later")
		}
		return fmt.Errorf("GitHub returned %s for %s", resp.Status, url)
	case http.StatusNotFound:
		return fmt.Errorf("not found on GitHub: %s", url)
	default:
		return fmt.Errorf("GitHub returned %s for %s", resp.Status, url)
	}
}
