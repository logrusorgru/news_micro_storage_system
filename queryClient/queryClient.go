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
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/gogo/protobuf/proto"

	"github.com/nats-io/nats.go"

	"github.com/logrusorgru/news_micro_storage_system/msg"
)

// defautls
const (
	Addr    = "127.0.0.1:3000"
	Timeout = 1 * time.Second
	NATSURL = nats.DefaultURL
	Subject = msg.Name
)

// A Config represents all storage configurations
type Config struct {

	// HTTP

	Addr    string        // address and port to listen on
	Timeout time.Duration // request timeout

	// NATS

	NATSURL string // nats url
	Subject string // nats subject name
}

// NewConfig with defaults
func NewConfig() (c *Config) {
	c = new(Config)
	c.Addr = Addr
	c.Timeout = Timeout
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
	flag.StringVar(&c.Addr,
		prefix+"addr",
		c.Addr,
		"listening address with port")
	flag.DurationVar(&c.Timeout,
		prefix+"timeout",
		c.Timeout,
		"HTTP request timeout")
	flag.StringVar(&c.NATSURL,
		prefix+"nats-url",
		c.NATSURL,
		"NATS server's url")
	flag.StringVar(&c.Subject,
		prefix+"nats-subject",
		c.Subject,
		"NATS subject's name")
}

// A Server represents HTTP server
type Server struct {
	Conf   *Config     // reference to Config
	Server http.Server // HTTP Server
	Conn   *nats.Conn  // NATS connection
}

// NewServer connects to NATS server and returns HTTP server.
// Start it using
//
//    srv.Server.ListenAndServe()
//
func NewServer(conf *Config) (srv *Server, err error) {
	srv = new(Server)
	srv.Conf = conf
	srv.Server.Addr = conf.Addr

	// setup NATS
	if srv.Conn, err = nats.Connect(conf.NATSURL); err != nil {
		return nil, fmt.Errorf("conencting NATS: %v", err)
	}

	// setup routes
	srv.setupRoutes()
	return
}

func (s *Server) setupRoutes() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)                  // request logs
	r.Use(middleware.Timeout(s.Conf.Timeout)) // request timeout
	r.Get("/news/{id}", s.getNews)
	s.Server.Handler = r
}

// GET /news/{id}
func (s *Server) getNews(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid news identifier: "+err.Error(), http.StatusBadRequest)
		return
	}
	if id < 0 {
		http.Error(w, "news identifier can't be negative", http.StatusBadRequest)
		return
	}
	var mid msg.ID
	mid.ID = id
	val, err := proto.Marshal(&mid)
	if err != nil {
		panic("encoding error: " + err.Error()) // must not happen
	}
	// NATS request
	resp, err := s.Conn.RequestWithContext(r.Context(), s.Conf.Subject, val)
	if err != nil {
		// 500 error
		log.Print("[NATS] request error: ", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	//
	var mrsp msg.Response
	if err = proto.Unmarshal(resp.Data, &mrsp); err != nil {
		panic("decoding error: " + err.Error())
	}
	if mrsp.Error != "" {
		// 404
		if mrsp.Error == sql.ErrNoRows.Error() {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		// 500 error
		log.Print("[NATS] request error: ", mrsp.Error)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	// found
	w.Header().Add("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(mrsp.Item); err != nil {
		log.Print("[HTTP] writing response: ", err)
	}
}

// Close the Server.
func (s *Server) Close() (err error) {
	err = s.Server.Close()
	s.Conn.Close()
	return
}
