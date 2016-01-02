// Copyright 2015-2016, Cyrill @ Schumacher.fm and the CoreStore contributors
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

//	Experimental micro service app to handle package store
// TODO: Add example for github.com/ebay/fabio

import (
	"net/http"
	"time"

	"github.com/corestoreio/csfw/config"
	"github.com/corestoreio/csfw/config/scope"
	"github.com/corestoreio/csfw/net/ctxhttp"
	"github.com/corestoreio/csfw/net/ctxjwt"
	"github.com/corestoreio/csfw/net/ctxrouter"
	"github.com/corestoreio/csfw/net/httputil"
	"github.com/corestoreio/csfw/storage/csdb"
	"github.com/corestoreio/csfw/storage/dbr"
	"github.com/corestoreio/csfw/store"
	"github.com/corestoreio/csfw/util/log"
	"golang.org/x/net/context"
)

const ServerAddress = "127.0.0.1:3010"

func init() {
	log.PkgLog = log.NewStdLogger(
		log.SetStdLevel(log.StdLevelInfo),
	)
	store.PkgLog = log.PkgLog
	config.PkgLog = log.PkgLog
	csdb.PkgLog = log.PkgLog
	dbr.PkgLog = log.PkgLog
}

type app struct {
	dbc    *dbr.Connection
	config *config.Service
	jwtSrv *ctxjwt.Service
}

// newApp creates a new application. function can only be called once
func newApp() *app {
	a := new(app)
	var err error

	// make sure env var CS_DSN is set and points to the appropriate database
	if a.dbc, err = csdb.Connect(); err != nil {
		log.Fatal("MySQL Connect", "err", err)
	}

	if err := config.TableCollection.Init(a.dbc.NewSession()); err != nil {
		log.Fatal("config.TableCollection.Init", "err", err)
	}
	if err := store.TableCollection.Init(a.dbc.NewSession()); err != nil {
		log.Fatal("store.TableCollection.Init", "err", err)
	}

	a.config = config.NewService(config.WithDBStorage(a.dbc.DB))

	// create JSON web token instance
	if a.jwtSrv, err = ctxjwt.NewService(); err != nil {
		log.Fatal("ctxjwt.NewService", "err", err)
	}
	a.jwtSrv.EnableJTI = true

	return a
}

func (a *app) close() {
	if err := a.dbc.Close(); err != nil {
		log.Fatal("MySQL Close", "err", err)
	}
}

func (a *app) routeLogin(rtr *ctxrouter.Router) {

	staticClaims := map[string]interface{}{
		"xfoo":  "bar",
		"xtime": time.Now().Unix(),
	}

	rtr.GET("/login", func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		ts, _, err := a.jwtSrv.GenerateToken(staticClaims)
		if err != nil {
			return err
		}
		return httputil.NewPrinter(w, r).WriteString(http.StatusOK, ts)
	})
}

func (a *app) setupStoreRoutes(rtr *ctxrouter.Router) {

	//	eg1 := e.Group(httputils.APIRoute.String(), a.jwtSrv.WithParseAndValidate())

	path := httputil.APIRoute.String() + store.RouteStores

	rtr.Handler("GET", path,
		ctxhttp.Chain(jsonStores, a.jwtSrv.WithParseAndValidate()),
	)

	//	eg1.Get(store.RouteStores, store.RESTStores(sm))
	//	eg1.POST(store.RouteStores, store.RESTStoreCreate)
	//	eg1.GET(store.RouteStore, store.RESTStore)
	//	eg1.PUT(store.RouteStore, store.RESTStoreSave)
	//	eg1.DELETE(store.RouteStore, store.RESTStoreDelete)
}

func jsonStores(ctx context.Context, w http.ResponseWriter, r *http.Request) error {

	storeReader, _, err := store.FromContextReader(ctx)
	if err != nil {
		return err // default StatusInternalServerError
	}

	stores, err := storeReader.Stores()
	if err != nil {
		return ctxhttp.NewErrorFromErrors(http.StatusInternalServerError, err)
	}
	return httputil.NewPrinter(w, r).JSON(http.StatusOK, stores)
}

func main() {
	a := newApp()
	defer a.close() // @todo check signal and close gracefully

	ctx := store.WithContextMustService(
		scope.Option{Website: scope.MockID(1)}, // run website ID 1, see database table, like Mage::run('code','store')
		store.MustNewStorage(store.WithDatabaseInit(a.dbc.NewSession())),
	)

	ctx = config.WithContextGetter(ctx, a.config)

	router := ctxrouter.New(ctx)
	a.routeLogin(router)
	a.setupStoreRoutes(router)

	router.Handle("GET", "/error", func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		return store.ErrContextServiceNotFound
	})

	println("Starting server @ ", ServerAddress)

	log.Fatal("ListenAndServe", "err", http.ListenAndServe(ServerAddress, router))

}
