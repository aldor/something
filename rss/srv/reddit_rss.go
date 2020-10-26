package srv

import (
	"net/http"
)

func (a *App) getRedditRSS(w http.ResponseWriter, r *http.Request) {
	feed := a.RedditRSSCache.GetFeed()
	w.Header().Set("Content-Type", "application/rss+xml")
	w.WriteHeader(http.StatusOK)
	feed.WriteRss(w)
}
