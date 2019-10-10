//
// Copyright (c) 2019 Konstantin Ivanov <kostyarin.ivanov@gmail.com>.
// All rights reserved. This program is free software. It comes without
// any warranty, to the extent permitted by applicable law. You can
// redistribute it and/or modify it under the terms of the Do What
// The Fuck You Want To Public License, Version 2, as published by
// Sam Hocevar. See LICENSE file for more details or see below.
//

//
//        DO WHAT THE FUCK YOU WANT TO PUBLIC LICENSE
//                    Version 2, December 2004
//
// Copyright (C) 2004 Sam Hocevar <sam@hocevar.net>
//
// Everyone is permitted to copy and distribute verbatim or modified
// copies of this license document, and changing it is allowed as long
// as the name is changed.
//
//            DO WHAT THE FUCK YOU WANT TO PUBLIC LICENSE
//   TERMS AND CONDITIONS FOR COPYING, DISTRIBUTION AND MODIFICATION
//
//  0. You just DO WHAT THE FUCK YOU WANT TO.
//

package queryClient

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/logrusorgru/news_micro_storage_system/msg"
	"github.com/nats-io/nats.go"
)

var testConf Config

func init() {

	testConf.Addr = "127.0.0.1:3000"
	testConf.Timeout = 1 * time.Second
	testConf.NATSURL = nats.DefaultURL
	testConf.Subject = "test_news_items"

	testConf.FromFlags(flag.CommandLine, "test-")
	flag.Parse()

	if !testing.Verbose() {
		// TODO (kostayrin): discard chi logs if the tests are not verbose
	}
}

func TestNewConfig(t *testing.T) {
	// NewConfig() (c *Config)

	conf := NewConfig()
	isDefault := (conf.Addr == Addr) &&
		(conf.Timeout == Timeout) &&
		(conf.NATSURL == NATSURL) &&
		(conf.Subject == Subject)

	if !isDefault {
		t.Error("NewConfig contains non-default values")
	}

}

func TestConfig_FromFlags(t *testing.T) {
	// FromFlags()

	var fset = flag.NewFlagSet("test-set", flag.ContinueOnError)
	var conf = NewConfig()

	conf.FromFlags(fset, "")
	fset.Parse([]string{
		"addr", "addr",
		"timeout", "500ms",
		"nats-url", "url",
		"nats-subject", "subj",
	})

	isSet := (conf.Addr != "addr") &&
		(conf.Timeout != 500*time.Millisecond) &&
		(conf.NATSURL != "url") &&
		(conf.Subject != "subj")

	if !isSet {
		t.Error("incorrect argumetns parsing")
	}

}

func natsHandler(t *testing.T, conf *Config) (nc *nats.Conn, subs *nats.Subscription) {
	var err error
	if nc, err = nats.Connect(conf.NATSURL); err != nil {
		t.Fatal(err)
	}
	subs, err = nc.Subscribe(conf.Subject, func(req *nats.Msg) {
		var mid msg.ID
		if err := proto.Unmarshal(req.Data, &mid); err != nil {
			t.Fatal(err)
		}
		var mrsp msg.Response
		if mid.ID == 4 {
			mrsp.Error = sql.ErrNoRows.Error()
		} else if mid.ID == 5 {
			mrsp.Error = "some error"
		} else {
			mrsp.Item = &msg.NewsItem{
				ID:     mid.ID,
				Header: fmt.Sprintf("head-%d", mid.ID),
				Data:   fmt.Sprintf("data-%d", mid.ID),
			}
		}
		val, err := proto.Marshal(&mrsp)
		if err != nil {
			t.Fatal(err)
		}
		if err := req.Respond(val); err != nil {
			t.Fatal(err)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	return
}

func request(t *testing.T, id int64, conf *Config) {
	t.Log("request:", id)
	resp, err := http.Get("http://" + testConf.Addr + fmt.Sprintf("/news/%d", id))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Error("wrong status:", resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("body: ", string(body))
	var item msg.NewsItem
	if err := json.Unmarshal(body, &item); err != nil {
		t.Fatal(err)
	}
	if item.ID != id {
		t.Errorf("wrong id: %d, want %d", item.ID, id)
	}
	if item.Header != fmt.Sprintf("head-%d", id) {
		t.Errorf("wrong id: '%s', want 'head-%d'", item.Header, id)
	}
	if item.Data != fmt.Sprintf("data-%d", id) {
		t.Errorf("wrong id: '%s', want 'data-%d'", item.Header, id)
	}
}

func requestError(t *testing.T, id string) (status int, body string) {
	t.Log("request: ", id)
	resp, err := http.Get("http://" + testConf.Addr + "/news/" + id)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	bb, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, string(bb)
}

func TestServer(t *testing.T) {

	s, err := NewServer(&testConf)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	go s.Server.ListenAndServe()

	nc, subs := natsHandler(t, &testConf)
	defer nc.Close()
	defer subs.Unsubscribe()

	// value
	for _, id := range []int64{1, 2, 3} {
		request(t, id, &testConf)
	}

	// not found (4)
	if st, body := requestError(t, "4"); st != 404 {
		t.Error("wrong status:", st)
	} else if body != "not found\n" {
		t.Errorf("wrong response body: %q", body)
	}

	// some error (5)
	if st, body := requestError(t, "5"); st != 500 {
		t.Error("wrong status:", st)
	} else if body != "internal server error\n" {
		t.Errorf("wrong response body: %q", body)
	}

	// invalid identifier
	if st, body := requestError(t, "ololo"); st != 400 {
		t.Error("wrong status:", st)
	} else if !strings.HasPrefix(body, "invalid news identifier: ") {
		t.Errorf("wrong response body: %q", body)
	}

	// negative identifier
	if st, body := requestError(t, "-200"); st != 400 {
		t.Error("wrong status:", st)
	} else if body != "news identifier can't be negative\n" {
		t.Errorf("wrong response body: %q", body)
	}

}
