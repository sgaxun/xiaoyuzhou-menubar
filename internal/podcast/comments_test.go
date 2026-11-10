package podcast

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
)

func TestCommentsAuthenticationAndPagination(t *testing.T) {
	cursor := Map{"id": testID, "hotSortScore": 12.5, "section": "NORMAL", "direction": "NEXT"}
	var device string
	calls := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("x-jike-access-token") != "test-token" {
			t.Error("missing token")
		}
		id := r.Header.Get("x-jike-device-id")
		if id == "" || (device != "" && id != device) {
			t.Error("missing or unstable device ID")
		}
		device = id
		var b Map
		_ = json.NewDecoder(r.Body).Decode(&b)
		if !reflect.DeepEqual(b["loadMoreKey"], cursor) {
			t.Errorf("cursor changed: %#v", b["loadMoreKey"])
		}
		if calls == 1 {
			if r.URL.Path != "/v1/comment/list-primary" || obj(b["owner"])["id"] != testID || b["order"] != "HOT" {
				t.Error("wrong primary request")
			}
		} else if r.URL.Path != "/v1/comment/list-thread" || b["primaryCommentId"] != testID || b["order"] != "TIME" {
			t.Error("wrong thread request")
		}
		writeJSON(w, Map{"data": []any{Map{"id": testID, "text": "hello"}}, "loadMoreKey": cursor})
	})
	for _, thread := range []string{"", testID} {
		v, err := c.ListComments(context.Background(), "test-token", testID, thread, cursor)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(v["loadMoreKey"], cursor) {
			t.Fatal("response cursor changed")
		}
	}
	if _, err := c.ListComments(context.Background(), "test-token", "bad", "", nil); err == nil {
		t.Fatal("invalid episode accepted")
	}
	if _, err := c.ListComments(context.Background(), "", testID, "", nil); err == nil {
		t.Fatal("missing token accepted")
	}
	if calls != 2 {
		t.Fatal("invalid requests reached server")
	}
}

func TestCommentSortOrders(t *testing.T) {
	for _, order := range []string{"HOT", "TIME", "TIMESTAMP"} {
		t.Run(order, func(t *testing.T) {
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				var b Map
				_ = json.NewDecoder(r.Body).Decode(&b)
				if b["order"] != order {
					t.Errorf("wrong order: %v", b["order"])
				}
				writeJSON(w, Map{"data": []any{}})
			})
			if _, err := c.ListComments(context.Background(), "test-token", testID, "", nil, order); err != nil {
				t.Fatal(err)
			}
		})
	}
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) { t.Fatal("invalid sort reached server") })
	if _, err := c.ListComments(context.Background(), "test-token", testID, "", nil, "INVALID"); err == nil {
		t.Fatal("invalid sort accepted")
	}
}
