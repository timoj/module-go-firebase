// Demo is a ThingsDB module which may be used as a template to build modules.
//
// This module simply extract a given `message` property from a request and
// returns this message.
//
// For example:
//
//     // Create the module (@thingsdb scope)
//     new_module('DEMO', 'demo', nil, nil);
//
//     // When the module is loaded, use the module in a future
//     future({
//       module: 'DEMO',
//       message: 'Hi ThingsDB module!',
//     }).then(|msg| {
//	      `Got the message back: {msg}`
//     });
//
package main

import (
	"context"
	"fmt"
	"log"
	"sync"

	timod "github.com/thingsdb/go-timod"

	firebase "firebase.google.com/go"
	"firebase.google.com/go/auth"
	"github.com/vmihailenco/msgpack"
	"google.golang.org/api/option"
)

var client *auth.Client = nil
var mux sync.Mutex

type serverSiriDB struct {
	Host string `msgpack:"host"`
	Port int    `msgpack:"port"`
}

type authFirebase struct {
	jsonData []byte `msgpack:"username"`
}

type reqValidateToken struct {
	Token string `msgpack:"query"`
}

func setupFirebase(authFB *authFirebase) {
	mux.Lock()
	defer mux.Unlock()

	if client != nil {
		client = nil
	}

	opt := option.WithCredentialsJSON(authFB.jsonData) //Firebase admin SDK initialization
	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		fmt.Sprintf("Got error while creating app: %s", err)
		panic("Firebase load error, could not create app")
	} //Firebase Auth
	client, err = app.Auth(context.Background())
	if err != nil {
		fmt.Sprintf("Got error while creating client: %s", err)
		panic("Firebase load error, could not create client")
	}
}

func handleValidateToken(pkg *timod.Pkg, req *reqValidateToken) {

	//verify token
	token, err := client.VerifyIDToken(context.Background(), req.Token)
	if err != nil {
		timod.WriteEx(
			pkg.Pid,
			timod.ExOperation,
			fmt.Sprintf("Validation has failed: %s", err))
		return
	}

	timod.WriteResponse(pkg.Pid, &token)
}

func onModuleReq(pkg *timod.Pkg) {
	mux.Lock()
	defer mux.Unlock()

	if client == nil {
		timod.WriteEx(
			pkg.Pid,
			timod.ExOperation,
			"Error: Firebase is not connected; please check the module configuration")
		return
	}

	var req reqValidateToken
	err := msgpack.Unmarshal(pkg.Data, &req)
	if err != nil {
		timod.WriteEx(
			pkg.Pid,
			timod.ExBadData,
			"Error: Failed to unpack Firebase token validation request")
		return
	}

	if req.Token == "" {
		timod.WriteEx(
			pkg.Pid,
			timod.ExBadData,
			"Error: Firebase token validations requires `token`")
		return
	}

	handleValidateToken(pkg, &req)
}

func handler(buf *timod.Buffer, quit chan bool) {
	for {
		select {
		case pkg := <-buf.PkgCh:
			switch timod.Proto(pkg.Tp) {
			case timod.ProtoModuleConf:
				var auth authFirebase
				err := msgpack.Unmarshal(pkg.Data, &auth)
				if err == nil {
					setupFirebase(&auth)
				} else {
					log.Println("Error: Missing or invalid SiriDB configuration")
					timod.WriteConfErr()
				}

			case timod.ProtoModuleReq:
				onModuleReq(pkg)

			default:
				log.Printf("Error: Unexpected package type: %d", pkg.Tp)
			}
		case err := <-buf.ErrCh:
			// In case of an error you probably want to quit the module.
			// ThingsDB will try to restart the module a few times if this
			// happens.
			log.Printf("Error: %s", err)
			quit <- true
		}
	}
}

func main() {
	// Starts the module
	timod.StartModule("siridb", handler)

	if client != nil {
		client = nil
	}
}
