package reddit

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

type UserSavedResponse struct {
	client    *Client
	data      userSavedResponse
	processed int
}

func (u *UserSavedResponse) NextPost() (*Post, error) {
	if u.processed >= u.data.Data.Dist {
		// TODO Make new request
		if len(u.data.Data.After) == 0 {
			return nil, nil
		}
		request := getUserSavedRequest{
			User:  u.client.cfg.Username,
			After: u.data.Data.After,
		}
		data, err := u.client.getUserSaved(request)
		if err != nil {
			return nil, fmt.Errorf("failed to get new posts")
		}
		u.processed = 0
		u.data = data
		// TODO: check data boundaries?
	}
	post := u.data.Data.Children[u.processed]
	u.processed++
	return &post, nil
}

type Post struct {
	Kind string `json:"kind"`
	Data map[string]interface{}
}

const (
	PostKind    = "t3"
	CommentKind = "t1"
)

type userSavedResponse struct {
	Kind string `json:"kind"`
	Data struct {
		After    string `json:"after"`
		Before   string `json:"before"`
		Dist     int    `json:"dist"`
		Children []Post `json:"children"`
	} `json:"data"`
}

type getUserSavedRequest struct {
	User  string
	After string
}

func (c *Client) GetUserSaved(user string) (UserSavedResponse, error) {
	result := UserSavedResponse{
		client: c,
	}
	rawSaved, err := c.getUserSaved(getUserSavedRequest{User: user})
	if err != nil {
		return result, fmt.Errorf("failed to get user saved: %s", err)
	}
	result.data = rawSaved
	return result, nil
}

func (c *Client) getUserSaved(req getUserSavedRequest) (userSavedResponse, error) {
	result := userSavedResponse{}
	rel := &url.URL{Path: fmt.Sprintf("/user/%s/saved", req.User)}
	u := c.url.ResolveReference(rel)
	request, err := http.NewRequest(
		"GET",
		u.String(),
		nil,
	)
	if err != nil {
		return result, fmt.Errorf("failed to create request: %s", err)
	}
	q := request.URL.Query()
	if len(req.After) > 0 {
		q.Add("after", req.After)
	}
	request.URL.RawQuery = q.Encode()
	token := c.tokenator.GetToken()
	request.Header.Add("Authorization", fmt.Sprintf("bearer %s", token))
	request.Header.Add("User-Agent", c.cfg.UserAgent)
	resp, err := c.client.Do(request)
	if err != nil {
		return result, fmt.Errorf("failed to make request: %s", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return result, fmt.Errorf("request finished with status %s: %s", resp.Status, body)
	}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return result, fmt.Errorf("failed to parse json: %s", err)
	}
	return result, nil
}
