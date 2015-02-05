// Copyright 2014, 2015 Zac Bergquist
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package spotify

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// User contains the basic, publicly available information about a Spotify user.
type User struct {
	// The name displayed on the user's profile.
	// Note: Spotify currently fails to populate
	// this field when querying for a playlist.
	DisplayName string `json:"display_name"`
	// Known public external URLs for the user.
	ExternalURLs ExternalURL `json:"external_urls"`
	// Information about followers of the user.
	Followers Followers `json:"followers"`
	// A link to the Web API endpoint for this user.
	Endpoint string `json:"href"`
	// The Spotify user ID for the user.
	ID string `json:"id"`
	// The user's profile image.
	Images []Image `json:"images"`
	// The Spotify URI for the user.
	URI URI `json:"uri"`
}

// PrivateUser contains additional information about a user.
// This data is private and requires user authentication.
type PrivateUser struct {
	User
	// The country of the user, as set in the user's account profile.
	// An ISO 3166-1 alpha-2 country code.  This field is only available when the
	// current user has granted acess to the ScopeUserReadPrivate scope.
	Country string `json:"country"`
	// The user's email address, as entered by the user when creating their account.
	// Note: this email is UNVERIFIED - there is no proof that it actually
	// belongs to the user.  This field is only available when the current user
	// has granted access to the ScopeUserReadEmail scope.
	Email string `json:"email"`
	// The user's Spotify subscription level: "premium", "free", etc.
	// The subscription level "open" can be considered the same as "free".
	// This field is only available when the current user has granted access to
	// the ScopeUserReadPrivate scope.
	Product string `json:"product"`
}

// GetUsersPublicProfile is a wrapper around DefaultClient.GetUsersPublicProfile.
func GetUsersPublicProfile(userID ID) (*User, error) {
	return DefaultClient.GetUsersPublicProfile(userID)
}

// GetUsersPublicProfile gets public profile information about a
// Spotify User.  It does not require authentication.
func (c *Client) GetUsersPublicProfile(userID ID) (*User, error) {
	spotifyURL := baseAddress + "users/" + string(userID)
	resp, err := c.http.Get(spotifyURL)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp.Body)
	}
	var user User
	err = json.NewDecoder(resp.Body).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// CurrentUser gets detailed profile information about the
// current user.  This call requires authorization.
//
// Reading the user's email address requires that the application
// has the ScopeUserReadEmail scope.  Reading the country, display
// name, profile images, and product subscription level requires
// that the application has the ScopeUserReadPrivate scope.
//
// Warning: The email address in the response will be the address
// that was entered when the user created their spotify account.
// This email address is unverified - do not assume that Spotify has
// checked that the email address actually belongs to the user.
func (c *Client) CurrentUser() (*PrivateUser, error) {
	if c.AccessToken == "" || c.TokenType != BearerToken {
		return nil, ErrAuthorizationRequired
	}
	spotifyURL := baseAddress + "me"
	req, err := c.newHTTPRequest("GET", spotifyURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp.Body)
	}
	var result PrivateUser
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// CurrentUsersTracks gets a list of songs saved in the current
// Spotify user's "Your Music" library.
func (c *Client) CurrentUsersTracks() (*SavedTrackPage, error) {
	return c.CurrentUsersTracksOpt(nil)
}

// CurrentUsersTracksOpt is like CurrentUsersTracks, but it accepts additional
// options for sorting and filtering the results.
func (c *Client) CurrentUsersTracksOpt(opt *Options) (*SavedTrackPage, error) {
	if c.AccessToken == "" || c.TokenType != BearerToken {
		return nil, ErrAuthorizationRequired
	}
	spotifyURL := baseAddress + "me/tracks"
	if opt != nil {
		v := url.Values{}
		if opt.Country != nil {
			v.Set("country", *opt.Country)
		}
		if opt.Limit != nil {
			v.Set("limit", strconv.Itoa(*opt.Limit))
		}
		if opt.Offset != nil {
			v.Set("offset", strconv.Itoa(*opt.Offset))
		}
		if params := v.Encode(); params != "" {
			spotifyURL += "?" + params
		}
	}
	req, err := c.newHTTPRequest("GET", spotifyURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp.Body)
	}
	var result SavedTrackPage
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// Follow adds the current user as a follower of one or more
// artists or other spotify users, identified by their Spotify IDs.
// This call requires authorization.
//
// Modifying the lists of artists or users the current user follows
// requires that the application has the ScopeUserFollowModify scope.
func (c *Client) Follow(ids ...ID) error {
	return c.modifyFollowers(true, ids...)
}

// Unfollow removes the current user as a follower of one or more
// artists or other Spotify users.  This call requires authorization.
//
// Modifying the lists of artists or users the current user follows
// requires that the application has the ScopeUserFollowModify scope.
func (c *Client) Unfollow(ids ...ID) error {
	return c.modifyFollowers(false, ids...)
}

// CurrentUserFollows checks to see if the current user is following
// one or more artists or other Spotify Users.  This call requires
// authorization, and that the application has the ScopeUserFollowRead
// scope.
//
// The t argument indicates the type of the IDs, and must be either
// "user" or "artist".
//
// The result is returned as a slice of bool values in the same order
// in which the IDs were specified.
func (c *Client) CurrentUserFollows(t string, ids ...ID) ([]bool, error) {
	if c.AccessToken == "" || c.TokenType != BearerToken {
		return nil, ErrAuthorizationRequired
	}
	if l := len(ids); l == 0 || l > 50 {
		return nil, errors.New("spotify: UserFollows supports 1 to 50 IDs")
	}
	if t != "artist" && t != "user" {
		return nil, errors.New("spotify: t must be 'artist' or 'user'")
	}
	spotifyURL := fmt.Sprintf("%sme/following/contains?type=%s&ids=%s",
		baseAddress, t, strings.Join(toStringSlice(ids), ","))
	req, err := c.newHTTPRequest("GET", spotifyURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp.Body)
	}
	var result []bool
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) modifyFollowers(follow bool, ids ...ID) error {
	if c.AccessToken == "" || c.TokenType != BearerToken {
		return ErrAuthorizationRequired
	}
	if l := len(ids); l == 0 || l > 50 {
		return errors.New("spotify: Follow/Unfollow supports 1 to 50 IDs")
	}
	spotifyURL := baseAddress + "me/following?" + strings.Join(toStringSlice(ids), ",")
	method := "PUT"
	if !follow {
		method = "DELETE"
	}
	req, err := c.newHTTPRequest(method, spotifyURL, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return decodeError(resp.Body)
	}
	return nil
}
