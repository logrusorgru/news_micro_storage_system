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
	"flag"
	"fmt"

	"github.com/gogo/protobuf/proto"
	_ "github.com/lib/pq"
	"github.com/nats-io/nats.go"

	"github.com/logrusorgru/news_micro_storage_system/msg"
)

// hardcoded
const (
	tableName = "news_items" // db table name
)

// defautls
const (
	DBAddr  = "localhost"
	DBPort  = 26257
	DBName  = msg.Name
	DBUser  = msg.Name
	NATSURL = nats.DefaultURL
	Subject = msg.Name
)

// Context represetns cacnelation with error.
type Context struct {
	Ctx    context.Context // golang context
	Cancel func()          // cancel the golang context
	Err    chan error      // terminating error or nil
}

// NewContext creates new Context that's obvious.
func NewContext() (c *Context) {
	c = new(Context)
	c.Ctx, c.Cancel = context.WithCancel(context.Background())
	// don't block if DB and NATS terminated with an error together
	c.Err = make(chan error, 2)
	return
}

// Terminate context with given error.
func (c *Context) Terminate(err error) {
	if err == context.Canceled {
		c.Err <- nil // no errors, a legal cancel
		return
	}
	c.Err <- err
	c.Cancel()
}

// Termiante with formated error.
func (c *Context) Terminatef(format string, args ...interface{}) {
	c.Terminate(fmt.Errorf(format, args...))
}

// Errs or nil if context was canceled by a legal reason.
// Only first error read has meaning.
func (c *Context) Errs() (err error) {
	select {
	case err = <-c.Err:
	default:
	}
	return
}

// A Config represents all storage configurations
type Config struct {

	// CockrouachDB

	DBAddr string // address (localhost)
	DBPort int    // port (26257)
	DBName string // databas name
	DBUser string // database user name

	// NATS

	NATSURL string // nats url
	Subject string // nats subject name
}

// NewConfig with defaults
func NewConfig() (c *Config) {
	c = new(Config)
	c.DBAddr = DBAddr
	c.DBPort = DBPort
	c.DBName = DBName
	c.DBUser = DBUser
	c.NATSURL = NATSURL
	c.Subject = Subject
	return
}

// FromFlags obtains config values from command-line flags.
// You should call flag.Parse after this method. Use
//
//     conf.FromFlags(flag.CommandLine, "")
//
// to use default (root) flag set.
//
// The prefix argument used to prefix all the flags with the
// given prefix. Use "prefix-" or something like that.
func (c *Config) FromFlags(fset *flag.FlagSet, prefix string) {
	flag.StringVar(&c.DBAddr,
		prefix+"db-addr",
		c.DBAddr,
		"cockroachdb server's address")
	flag.IntVar(&c.DBPort,
		prefix+"db-port",
		c.DBPort,
		"cockroachdb server's port")
	flag.StringVar(&c.DBName,
		prefix+"db-name",
		c.DBName,
		"database name")
	flag.StringVar(&c.DBUser,
		prefix+"db-user",
		c.DBUser,
		"database user name")
	flag.StringVar(&c.NATSURL,
		prefix+"nats-url",
		c.NATSURL,
		"NATS server's url")
	flag.StringVar(&c.Subject,
		prefix+"nats-subject",
		c.Subject,
		"NATS subject's name")
}

// OpenDBURL based on values of the Config.
func (c *Config) OpenDBURL() string {
	return fmt.Sprintf("postgresql://%s@%s:%d/%s?sslmode=disable",
		c.DBUser, c.DBAddr, c.DBPort, c.DBName)
}

type DB struct {
	DB *sql.DB // undelying SQL databse instance
}

// NewDB creates new conented DB instance.
func NewDB(conf *Config) (db *DB, err error) {
	db = new(DB)
	db.DB, err = sql.Open("postgres", conf.OpenDBURL())
	// TODO (kostyarin): configure db: max idle, max open, lifetime, etc
	//                   using hardcoded values, or keeping the values in
	//                   the Config
	return
}

// Init database creating table if it doesn't exist
func (db *DB) Init(ctx *Context) (err error) {
	const createTable = `CREATE TABLE IF NOT EXISTS ` + tableName + ` (
		id     SERIAL PRIMARY KEY,
		header VARCHAR(255),
		data   TEXT
	)`
	_, err = db.DB.ExecContext(ctx.Ctx, createTable)
	return
}

// Select news item by id. It returns sql.ErrNoRows if
// the requested item doesn't exist.
func (db *DB) Select(
	ctx *Context,
	id int64,
) (
	ni *msg.NewsItem,
	err error,
) {

	const selectNewsItem = `SELECT * FROM ` + tableName + ` WHERE id = $1`

	ni = new(msg.NewsItem)
	err = db.DB.QueryRowContext(ctx.Ctx, selectNewsItem, id).Scan(
		&ni.ID,
		&ni.Header,
		&ni.Data,
	)
	return
}

// Close the DB.
func (db *DB) Close() error {
	return db.DB.Close()
}

// The QQ represents NATS conenction and processor
type QQ struct {
	Conn *nats.Conn         // connection
	Subs *nats.Subscription // subscription
}

// NewQQ creates new connected, subscribed and handled.
func NewQQ(ctx *Context, conf *Config, db *DB) (qq *QQ, err error) {
	qq = new(QQ)
	if qq.Conn, err = nats.Connect(conf.NATSURL); err != nil {
		return nil, fmt.Errorf("conencting NATS: %v", err)
	}
	qq.Subs, err = qq.Conn.Subscribe(
		conf.Subject, // strings.Replace(conf.Subject, "_", ".", -1)
		qq.handler(ctx, db),
	)
	if err != nil {
		qq.Conn.Close()
		return nil, fmt.Errorf("subscribing '%s' subject: %v", conf.Subject, err)
	}
	return
}

// handler for requests.
func (qq *QQ) handler(ctx *Context, db *DB) func(req *nats.Msg) {
	return func(req *nats.Msg) {
		var (
			id  msg.ID
			err error
		)
		if err = proto.Unmarshal(req.Data, &id); err != nil {
			// should never happen
			ctx.Terminatef("[FATAL] NATS decoding received message: %v", err)
			return
		}
		var ni *msg.NewsItem
		ni, err = db.Select(ctx, id.ID)
		var rsp msg.Response
		rsp.Item = ni
		if err != nil {
			rsp.Error = err.Error()
		}
		val, err := proto.Marshal(&rsp)
		if err != nil {
			// must never happen
			panic("encoding msg.Response: " + err.Error())
		}
		if err = req.Respond(val); err != nil {
			// TODO (kostyarin): 1. do all this errors fatal?
			//                   2. what happens where the requester disappears
			ctx.Terminatef("[FATAL] NATS respnding message: %v", err)
			return
		}
	}
}

// Close the QQ.
func (qq *QQ) Close() (err error) {
	err = qq.Subs.Unsubscribe()
	qq.Conn.Close() // no error herer
	return
}
