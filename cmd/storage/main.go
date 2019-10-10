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

package main

import (
	"flag"
	"log"
	"os"
	"os/signal"

	"github.com/logrusorgru/news_micro_storage_system/storage"
)

func waitSigInt(ctx *storage.Context) {

	sigs := make(chan os.Signal, 2)
	signal.Notify(sigs, os.Interrupt)

	log.Print("got signal %s, exiting...", <-sigs)
	ctx.Cancel()
}

func main() {

	log.SetOutput(os.Stdout)

	conf := storage.NewConfig()
	conf.FromFlags(flag.CommandLine, "")
	flag.Parse()

	ctx := storage.NewContext()

	defer func() {
		if err := ctx.Errs(); err != nil {
			log.Fatal(err) // for the exit code
		}
	}()

	db, err := storage.NewDB(conf)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	qq, err := storage.NewQQ(ctx, conf, db)
	if err != nil {
		log.Fatal(err)
	}
	defer qq.Close()

	waitSigInt(ctx)
}
