package podcast

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"
)

var commentDevice struct {
	sync.Once
	id  string
	err error
}

// ListComments reads one server page. Cursors must be passed back unchanged.
func (c *Client) ListComments(ctx context.Context, token, eid, thread string, cursor any, orders ...string) (Map, error) {
	if !IsValidID(eid) || (thread != "" && !IsValidID(thread)) {
		return nil, fmt.Errorf("评论 ID 无效")
	}
	if token == "" {
		return nil, fmt.Errorf("请先到设置扫码登录，再查看评论")
	}
	commentDevice.Do(func() {
		var b [16]byte
		_, commentDevice.err = rand.Read(b[:])
		b[6] = (b[6] & 0x0f) | 0x40
		b[8] = (b[8] & 0x3f) | 0x80
		commentDevice.id = fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
	})
	if commentDevice.err != nil {
		return nil, commentDevice.err
	}
	order := "HOT"
	if len(orders) > 0 && orders[0] != "" {
		order = orders[0]
	}
	if order != "HOT" && order != "TIME" && order != "TIMESTAMP" {
		return nil, fmt.Errorf("评论排序无效")
	}
	path := "/v1/comment/list-primary"
	body := Map{"owner": Map{"id": eid, "type": "EPISODE"}, "order": order}
	if thread != "" {
		path = "/v1/comment/list-thread"
		body = Map{"primaryCommentId": thread, "order": "TIME"}
	}
	if cursor != nil {
		body["loadMoreKey"] = cursor
	}
	v, err := c.apiWithHeaders(ctx, path, token, "POST", body, map[string]string{"x-jike-device-id": commentDevice.id})
	if err != nil {
		return nil, err
	}
	result := obj(v)
	if _, valid := result["data"].([]any); !valid {
		return nil, fmt.Errorf("评论响应格式异常")
	}
	return Map{"data": result["data"], "loadMoreKey": result["loadMoreKey"], "totalCount": result["totalCount"]}, nil
}
