package srv

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	log "github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"

	"github.com/aldor/something/reddit"
	"github.com/aldor/something/rss/srv/cache"
)

// App holds application components and configs
type App struct {
	RedditClient   *reddit.Client
	Config         Config
	DB             *bolt.DB
	RedditRSSCache *cache.RedditRSS
	router         chi.Router
	server         *http.Server
	ctx            context.Context
	cancelCtx      context.CancelFunc
}

// Config is application configurations
type Config struct {
	Reddit struct {
		App          reddit.AppConfig `yaml:"app"`
		RSSAccessKey string           `yaml:"rss-access-key"`
	} `yaml:"reddit"`
	AdminKey string `yaml:"admin-key"`
}

// Serve launches server listening given addr
func (a *App) Serve(addr string) error {
	a.server = &http.Server{Addr: addr, Handler: a.router}
	return a.server.ListenAndServe()
}

// Shutdown gracefully shutdown application
func (a *App) Shutdown(ctx context.Context) {
	log.Info("shutting down app")
	err := a.server.Shutdown(ctx)
	if err != nil {
		log.Errorf("shutdown server: %s", err)
	}
	a.cancelCtx()
	a.RedditClient.Close()
	a.RedditRSSCache.Close()
	err = a.DB.Close()
	if err != nil {
		log.Errorf("close boltdb: %s", err)
	}
}

// NewApp creates new service instance
func NewApp(cfg Config, dataPath string) (*App, error) {
	ctx, cancel := context.WithCancel(context.Background())
	redditClient, err := reddit.NewClient(ctx, cfg.Reddit.App)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create reddit client: %s", err)
	}
	db, err := bolt.Open(dataPath, 0600, nil)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("open boltdb: %s", err)
	}
	redditRSSCache, err := cache.NewRedditRSSCache(ctx, redditClient, db, cfg.Reddit.App.Username)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create reddit rss cache: %s", err)
	}

	app := &App{
		RedditClient:   redditClient,
		Config:         cfg,
		DB:             db,
		RedditRSSCache: redditRSSCache,
		ctx:            ctx,
		cancelCtx:      cancel,
	}
	app.router = newRouter(app)
	log.Info("app created")
	return app, nil

}

func newRouter(app *App) chi.Router {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Get(fmt.Sprintf("/rss/reddit/%s", app.Config.Reddit.RSSAccessKey), app.getRedditRSS)

	return r
}
