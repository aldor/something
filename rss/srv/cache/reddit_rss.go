package cache

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/feeds"
	log "github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"

	"github.com/aldor/something/reddit"
)

const postsContentBucket = "RedditPostsContent"
const postsIndexBucket = "RedditPostsIndex"

// RedditRSS holds and updates feed of saved by user rss posts
type RedditRSS struct {
	client    *reddit.Client
	db        *bolt.DB
	user      string
	feed      *feeds.Feed
	rwlock    sync.RWMutex
	wg        sync.WaitGroup
	ctx       context.Context
	cancelCtx context.CancelFunc
}

// NewRedditRSSCache creates new RedditRss cache instance
func NewRedditRSSCache(
	ctx context.Context, client *reddit.Client, db *bolt.DB, user string,
) (*RedditRSS, error) {
	cache := RedditRSS{
		client: client,
		db:     db,
		user:   user,
	}
	cache.ctx, cache.cancelCtx = context.WithCancel(ctx)
	err := cache.update()
	if err != nil {
		return nil, fmt.Errorf("init cache: %s", err)
	}
	updateIn := 15 * time.Minute
	cache.wg.Add(1)
	go func() {
		defer cache.wg.Done()
		for {
			log.Info("reddit rss cache: sleep for ", updateIn)
			select {
			case <-cache.ctx.Done():
				log.Info("reddit rss cache: cancelled")
				return
			case <-time.NewTimer(updateIn).C:
				err := cache.update()
				if err != nil {
					log.Errorf("reddit rss cache: update failed: %s", err)
				}
			}
		}
	}()

	return &cache, nil
}

func (c *RedditRSS) update() error {
	savedPosts, err := c.client.GetUserSaved(c.user)
	if err != nil {
		return fmt.Errorf("get user saved posts: %s", err)
	}
	err = c.db.Update(func(tx *bolt.Tx) error {
		index, err := tx.CreateBucketIfNotExists([]byte(postsIndexBucket))
		if err != nil {
			return err
		}
		posts, err := tx.CreateBucketIfNotExists([]byte(postsContentBucket))
		if err != nil {
			return err
		}
		newPosts, err := getNewPosts(index, savedPosts)
		if err != nil {
			return fmt.Errorf("get new posts: %s", err)
		}
		log.Info("total new posts: ", len(newPosts))
		for _, post := range newPosts {
			id := getID(post)
			if id == nil {
				return fmt.Errorf("failed to get post id %v", post)
			}
			value, err := json.Marshal(post)
			redditID := []byte(*id)
			if err != nil {
				return fmt.Errorf("failed to marshal post %s: %s", redditID, err)
			}
			postIDRaw, err := posts.NextSequence()
			if err != nil {
				return fmt.Errorf("get next post id: %s", err)
			}
			postID := itob(postIDRaw)
			err = posts.Put(postID, value)
			if err != nil {
				return fmt.Errorf("put new post %s: %s", redditID, err)
			}
			err = index.Put(redditID, postID)
			if err != nil {
				return fmt.Errorf("put new post id: %s", err)
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("update posts cache: %s", err)
	}
	err = c.buildPostsFeed()
	if err != nil {
		return fmt.Errorf("build redit posts rss: %s", err)
	}
	return nil
}

func (c *RedditRSS) Close() {
	c.cancelCtx()
	c.wg.Wait()
}

func getNewPosts(postsIndex *bolt.Bucket, savedPosts reddit.UserSavedResponse) ([]*reddit.Post, error) {
	newPosts := make([]*reddit.Post, 0, 8)
	for {
		post, err := savedPosts.NextPost()
		if err != nil {
			return nil, fmt.Errorf("get saved post: %s", err)
		}
		if post == nil {
			// no more posts
			break
		}
		id := getID(post)
		if id == nil {
			log.Errorf("got post with nil id: %s", post)
			continue
		}
		if postsIndex.Get([]byte(*id)) != nil {
			break
		}
		// post is missing in db
		newPosts = append(newPosts, post)
		continue
	}
	return newPosts, nil
}

// GetFeed return gorilla/feeds.Feed with posts
func (c *RedditRSS) GetFeed() *feeds.Feed {
	c.rwlock.RLock()
	defer c.rwlock.RUnlock()
	return c.feed
}

func (c *RedditRSS) buildPostsFeed() error {
	feed := &feeds.Feed{
		Title: fmt.Sprintf("%s's reddit saved posts", c.user),
		Link: &feeds.Link{
			Href: fmt.Sprintf("https://www.reddit.com/user/%s/saved/", c.user),
		},
	}
	err := c.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(postsContentBucket))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			post := &reddit.Post{}
			err := json.Unmarshal(v, post)
			if err != nil {
				return fmt.Errorf("failed to unmarshal post %s: %s", k, err)
			}
			title := getPostTitle(post)
			link := getPostLink(post)
			// log.Infof("post title %s", title)
			feed.Add(&feeds.Item{
				Title: title,
				Link:  &feeds.Link{Href: link},
			})
		}
		return nil
	})
	if err != nil {
		return err
	}
	c.rwlock.Lock()
	defer c.rwlock.Unlock()
	c.feed = feed
	return nil
}

// itob returns an 8-byte big endian representation of v
func itob(v uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}

func getStringField(value map[string]interface{}, name string) *string {
	field, ok := value[name]
	if !ok {
		return nil
	}
	str, ok := field.(string)
	if !ok {
		return nil
	}
	return &str
}

func getID(p *reddit.Post) *string {
	return getStringField(p.Data, "id")
}

func getPostTitle(p *reddit.Post) string {
	if p.Kind == reddit.PostKind {
		title := getStringField(p.Data, "title")
		if title != nil {
			return *title
		}
		return "Reddit post"
	}
	if p.Kind == reddit.CommentKind {
		title := getStringField(p.Data, "link_title")
		if title != nil {
			return *title
		}
		return "comment"
	}
	log.Errorf("unexpected reddit post kind: %s", p.Kind)
	return "Reddit post"
}

func getPostLink(p *reddit.Post) string {
	if p.Kind == reddit.PostKind {
		link := getStringField(p.Data, "permalink")
		if link != nil {
			return fmt.Sprintf("https://reddit.com%s", *link)
		}
		return ""
	}
	if p.Kind == reddit.CommentKind {
		link := getStringField(p.Data, "link_permalink")
		if link != nil {
			return *link
		}
		return ""
	}
	log.Errorf("unexpected post kind: %s", p.Kind)
	return ""
}
