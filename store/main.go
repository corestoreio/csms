// Copyright 2015, Cyrill @ Schumacher.fm and the CoreStore contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

/*
	Experimental micro service app to handle package store
*/

import (
	"net/http"
	"time"

	"github.com/corestoreio/csfw/config"
	"github.com/corestoreio/csfw/net"
	"github.com/corestoreio/csfw/storage/csdb"
	"github.com/corestoreio/csfw/storage/dbr"
	"github.com/corestoreio/csfw/store"
	"github.com/corestoreio/csfw/user/userjwt"
	"github.com/corestoreio/csfw/utils/log"
	"github.com/labstack/echo"
)

const ServerAddress = "127.0.0.1:3010"

func init() {
	log.Set(log.NewStdLogger())
}

func main() {
	dbc, err := csdb.Connect()
	if err != nil {
		log.Fatal("MySQL Connect", "err", err)
	}
	defer dbc.Close() // @todo check signal and close gracefully

	if err := config.DefaultManager.ApplyCoreConfigData(dbc.NewSession()); err != nil {
		log.Fatal("config.DefaultManager.ApplyCoreConfigData", "err", err)
	}

	e := echo.New()
	e.SetHTTPErrorHandler(net.RESTErrorHandler)

	//	e.Use(mw.Logger())
	//e.Use(mw.Recover())
	//	e.SetDebug(true)

	eg1 := setupAuth(e)
	setupStoreRoutes(eg1, dbc.NewSession())

	println("Starting server @ ", ServerAddress)
	e.Run(ServerAddress)
}

func setupAuth(e *echo.Echo) *echo.Group {
	jwtMng, err := userjwt.New()
	if err != nil {
		log.Fatal("userjwt.New", "err", err)
	}
	jwtMng.EnableJTI = true
	e.Get("/login", routeLogin(jwtMng))
	return e.Group(net.APIRoute.String(), jwtMng.Authorization)

}

// just hacked into it. @todo: auth from admin_user table and role checking
func routeLogin(jm *userjwt.AuthManager) echo.HandlerFunc {

	staticClaims := map[string]interface{}{
		"xfoo":  "bar",
		"xtime": time.Now().Unix(),
	}

	return func(c *echo.Context) error {
		ts, _, err := jm.GenerateToken(staticClaims)
		if err != nil {
			return err
		}

		return c.String(http.StatusOK, ts)
	}
}

func setupStoreRoutes(g *echo.Group, dbrSess *dbr.Session) {
	sm := store.NewManager(
		store.NewStorageOption(),
	)
	if err := sm.ReInit(dbrSess); err != nil {
		log.Fatal("sm.ReInit(dbc.NewSession())", "err", err)
	}

	g.Get(store.RouteStores, store.RESTStores(sm))
	//	eg1.POST(store.RouteStores, store.RESTStoreCreate)
	//	eg1.GET(store.RouteStore, store.RESTStore)
	//	eg1.PUT(store.RouteStore, store.RESTStoreSave)
	//	eg1.DELETE(store.RouteStore, store.RESTStoreDelete)

}
