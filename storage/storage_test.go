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

package storage

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	_ "github.com/lib/pq"
	"github.com/logrusorgru/news_micro_storage_system/msg"
	"github.com/nats-io/nats.go"
)

var (
	testConf Config
	testIDs  []int64
)

func init() {

	testConf.DBAddr = "localhost"
	testConf.DBPort = 26257
	testConf.DBName = "test_news_items"
	testConf.DBUser = "test_news_items"
	testConf.NATSURL = nats.DefaultURL
	testConf.Subject = "test_news_items"

	testConf.FromFlags(flag.CommandLine, "test-")
	flag.Parse()
}

// open/fill/close
func fillupTestDB(t *testing.T) {
	db, err := sql.Open("postgres", testConf.OpenDBURL())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	const createTable = `CREATE TABLE IF NOT EXISTS ` + tableName + ` (
		id     SERIAL PRIMARY KEY,
		header VARCHAR(255),
		data   TEXT
	)`

	if _, err = db.Exec(createTable); err != nil {
		t.Fatal(err)
	}

	const insertTest = `INSERT INTO ` + tableName + ` (header, data) VALUES
		('one', 'one-data'),
		('two', 'two-data'),
		('three', 'three-data')
	RETURNING id
	`

	rows, err := db.Query(insertTest)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		testIDs = append(testIDs, id)
	}
}

//
// Context
//

func TestNewContext(t *testing.T) {
	// NewContext() (c *Context)

	if ctx := NewContext(); ctx.Ctx == nil {
		t.Error("missing context")
	} else if ctx.Cancel == nil {
		t.Error("missing cancel")
	} else if ctx.Err == nil {
		t.Error("missing channel")
	} else if cap(ctx.Err) < 2 {
		t.Error("channel's buffer is too short")
	}
}

func TestContext_Terminate(t *testing.T) {
	// Terminate(err error)

	var ctx = NewContext()

	ctx.Terminate(context.Canceled)

	if err := ctx.Errs(); err != nil {
		t.Error("unexpected error:", err)
	}

	select {
	case <-ctx.Ctx.Done():
		t.Error("context canceled")
	default:
	}

	ctx = NewContext()

	var testErr = errors.New("test error")

	ctx.Terminate(testErr)

	if err := ctx.Errs(); err != testErr {
		t.Error("unexpected error:", err)
	} else if err = ctx.Errs(); err != nil {
		t.Error("unexpected second error:", err)
	}

	select {
	case <-ctx.Ctx.Done():
	default:
		t.Error("context not canceled")
	}

}

func TestContext_Terminatef(t *testing.T) {
	// Terminatef(format string, args ...interface{})

	var ctx = NewContext()

	ctx.Terminatef("some error %d", 1)

	if err := ctx.Errs(); err == nil {
		t.Error("missing error")
	} else if err.Error() != "some error 1" {
		t.Error("unexpected error:", err)
	}

	select {
	case <-ctx.Ctx.Done():
	default:
		t.Error("context not canceled")
	}

}

func TestContext_Errs(t *testing.T) {
	// Errs() (err error)

	var ctx = NewContext()

	ctx.Terminatef("one")
	ctx.Terminatef("two")

	if err := ctx.Errs(); err == nil {
		t.Error("misising error")
	} else if err.Error() != "one" {
		t.Error("unexpected error:", err)
	}

	if err := ctx.Errs(); err == nil {
		t.Error("misising error")
	} else if err.Error() != "two" {
		t.Error("unexpected error:", err)
	}

	if err := ctx.Errs(); err != nil {
		t.Error("unexpected error:", err)
	}

}

//
// Config
//

func TestNewConfig(t *testing.T) {
	// NewConfig() (c *Config)

	conf := NewConfig()
	isDefault := (conf.DBAddr == DBAddr) &&
		(conf.DBPort == DBPort) &&
		(conf.DBName == DBName) &&
		(conf.DBUser == DBUser) &&
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
		"db-addr", "addr",
		"db-port", "10",
		"db-name", "name",
		"db-user", "user",
		"nats-url", "url",
		"nats-subject", "subj",
	})

	isSet := (conf.DBAddr != "addr") &&
		(conf.DBPort != 10) &&
		(conf.DBName != "name") &&
		(conf.DBUser != "user") &&
		(conf.NATSURL != "url") &&
		(conf.Subject != "subj")

	if !isSet {
		t.Error("incorrect argumetns parsing")
	}

}

func TestConfig_OpenDBURL(t *testing.T) {
	// OpenDBURL() string

	var conf = NewConfig()

	conf.DBAddr = "addr"
	conf.DBPort = 10
	conf.DBName = "name"
	conf.DBUser = "user"

	if conf.OpenDBURL() != "postgresql://user@addr:10/name?sslmode=disable" {
		t.Error("wrong DB path")
	}

}

//
// DB
//

func TestNewDB(t *testing.T) {
	// NewDB(conf *Config) (db *DB, err error)

	db, err := NewDB(&testConf)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if db.DB == nil {
		t.Error("missing *sql.DB instance")
	}

}

func testShouldHaveTable(t *testing.T) {
	db, err := sql.Open("postgres", testConf.OpenDBURL())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// should not have any error
	rows, err := db.Query(`SELECT * FROM ` + tableName)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	// ignore result
}

func TestDB_Init(t *testing.T) {
	// Init(ctx *Context) (err error)

	db, err := NewDB(&testConf)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var ctx = NewContext()

	if err := db.Init(ctx); err != nil {
		t.Fatal(err)
	}

	db.Close()             // fo the testShouldHaveTable
	testShouldHaveTable(t) //
}

func TestDB_Select(t *testing.T) {
	// Select(	ctx *Context, id int64) (ni *msg.NewsItem, err error)

	fillupTestDB(t)

	db, err := NewDB(&testConf)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var ctx = NewContext()

	if err := db.Init(ctx); err != nil {
		t.Fatal(err)
	}

	for i, id := range testIDs {
		ni, err := db.Select(ctx, id)
		if err != nil {
			t.Error(err)
		}
		switch i {
		case 0:
			if ni.Header != "one" && ni.Data == "one-data" {
				t.Error("wrong ni:", ni)
			}
		case 1:
			if ni.Header != "two" && ni.Data == "two-data" {
				t.Error("wrong ni:", ni)
			}
		case 2:
			if ni.Header != "three" && ni.Data == "three-data" {
				t.Error("wrong ni:", ni)
			}
		default:
			t.Error("inexpected case:", ni, err)
		}
	}

}

func TestDB_Close(t *testing.T) {
	// Close() error

	db, err := NewDB(&testConf)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Error(err)
	}
	// should be ok twice
	if err := db.Close(); err != nil {
		t.Error(err)
	}

}

//
// QQ
//

func requestNats(t *testing.T, conf *Config) {

	conn, err := nats.Connect(conf.NATSURL)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	var (
		mid  msg.ID
		mrsp msg.Response
	)

	// ids already filled up
	for i, id := range append(testIDs, 90210) {
		mid.ID = id
		req, err := proto.Marshal(&mid)
		if err != nil {
			t.Fatal("encoding error:", err)
		}
		resp, err := conn.Request(conf.Subject, req, 1*time.Second)
		if err != nil {
			t.Fatal("request error:", err)
		}
		if err := proto.Unmarshal(resp.Data, &mrsp); err != nil {
			t.Fatal("decoding error:", err)
		}
		switch i {
		case 0:
			if mrsp.Error != "" {
				t.Error("unexpected error:", mrsp.Error)
			} else if mrsp.Item == nil {
				t.Error("missing item")
			} else if mrsp.Item.Header != "one" && mrsp.Item.Data == "one-data" {
				t.Error("wrong ni:", mrsp.Item)
			}
		case 1:
			if mrsp.Error != "" {
				t.Error("unexpected error:", mrsp.Error)
			} else if mrsp.Item == nil {
				t.Error("missing item")
			} else if mrsp.Item.Header != "two" && mrsp.Item.Data == "two-data" {
				t.Error("wrong ni:", mrsp.Item)
			}
		case 2:
			if mrsp.Error != "" {
				t.Error("unexpected error:", mrsp.Error)
			} else if mrsp.Item == nil {
				t.Error("missing item")
			} else if mrsp.Item.Header != "three" && mrsp.Item.Data == "three-data" {
				t.Error("wrong ni:", mrsp.Item)
			}
		case 3:
			if mrsp.Error == "" {
				t.Error("missing error")
			} else if mrsp.Error != sql.ErrNoRows.Error() {
				t.Error("unexpected error:", mrsp.Error)
			}
		default:
			t.Errorf("%d (%d): inexpected case: %v", i, id, mrsp)
		}
	}

}

func TestNewQQ(t *testing.T) {
	// NewQQ(ctx *Context, conf *Config, db *DB) (qq *QQ, err error)

	var (
		ctx     = NewContext()
		db, err = NewDB(&testConf)
		qq      *QQ
	)

	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if qq, err = NewQQ(ctx, &testConf, db); err != nil {
		t.Error(err)
		return
	}
	defer qq.Close()

	requestNats(t, &testConf)

}

func TestQQ_handler(t *testing.T) {
	// handler(ctx *Context) func(req *nats.Msg)

	//

}

func TestQQ_Close(t *testing.T) {
	// Close() (err error)

	//

}
