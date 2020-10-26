package reddit

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

const getTokenURL = "https://www.reddit.com/api/v1/access_token"

type tokenator struct {
	token     string
	client    http.Client
	rwlock    sync.RWMutex
	cfg       AppConfig
	wg        sync.WaitGroup
	ctx       context.Context
	cancelCtx context.CancelFunc
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

func (t *tokenator) GetToken() string {
	t.rwlock.RLock()
	defer t.rwlock.RUnlock()
	return t.token
}

func (t *tokenator) StartUpdater(ctx context.Context) error {
	expiresIn, err := t.UpdateToken()
	if err != nil {
		return fmt.Errorf("failed to init reddit token: %s", err)
	}
	t.ctx, t.cancelCtx = context.WithCancel(ctx)
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		for {
			sleepFor := (expiresIn / 2)
			log.Infof("reddit token updater sleeps for %s", sleepFor)
			select {
			case <-t.ctx.Done():
				log.Info("reddit token updated cancelled")
				return
			case <-time.NewTimer(sleepFor).C:
			}
			expiresIn, err = t.UpdateToken()
			if err != nil {
				sleepFor := time.Duration(10) * time.Second
				log.Errorf("failed to update reddit token: %s, sleeping for %s seconds", sleepFor, err)
				select {
				case <-t.ctx.Done():
					log.Info("reddit token updated cancelled")
					return
				case <-time.NewTimer(sleepFor).C:
				}
				continue
			}
		}
	}()
	return nil
}

func (t *tokenator) UpdateToken() (time.Duration, error) {
	var zeroTime time.Duration
	t.rwlock.Lock()
	defer t.rwlock.Unlock()
	log.Info("Getting new reddit oauth token")
	request, err := http.NewRequest(
		"POST",
		getTokenURL,
		strings.NewReader(
			fmt.Sprintf(
				"grant_type=password&username=%s&password=%s",
				t.cfg.Username,
				t.cfg.Password,
			),
		),
	)
	if err != nil {
		return zeroTime, fmt.Errorf("failed to create request %s", err)
	}
	request.Header.Add("User-Agent", t.cfg.UserAgent)
	request.SetBasicAuth(
		t.cfg.AppID, t.cfg.AppSecret,
	)
	resp, err := t.client.Do(request)
	if err != nil {
		return zeroTime, fmt.Errorf("failed to do request %s", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return zeroTime, fmt.Errorf("failed to read body %s", err)
	}
	if resp.StatusCode != http.StatusOK {
		return zeroTime, fmt.Errorf(
			"request finished with status %v, body: %s",
			resp.StatusCode,
			string(body), // TODO: truncate
		)
	}
	tokenResp := tokenResponse{}
	err = json.Unmarshal(body, &tokenResp)
	if err != nil {
		return zeroTime, fmt.Errorf("failed to parse body %s", err)
	}
	t.token = tokenResp.AccessToken
	return time.Duration(tokenResp.ExpiresIn) * time.Second, nil
}

func (t *tokenator) Close() {
	t.cancelCtx()
	t.wg.Wait()
}
