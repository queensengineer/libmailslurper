// Copyright 2013-2014 Adam Presley. All rights reserved
// Use of this source code is governed by the MIT license
// that can be found in the LICENSE file.

package server

import (
	"log"
	"net"

	"github.com/mailslurper/libmailslurper/model/mailitem"
	"github.com/mailslurper/libmailslurper/receiver"
)

/*
Establishes a listening connection to a socket on an address. This will
return a net.Listener handle.
*/
func SetupSmtpServerListener(address string) (*net.TCPListener, error) {
	result := &net.TCPListener{}

	tcpAddress, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return result, err
	}

	log.Println("libmailslurper: INFO - SMTP listener running on", address)
	return net.ListenTCP("tcp", tcpAddress)
}

/*
Closes a socket connection in an Server object. Most likely used in a defer call.
*/
func CloseSmtpServerListener(handle *net.TCPListener) {
	handle.Close()
}

/*
This function starts the process of handling SMTP client connections.
The first order of business is to setup a channel for writing
parsed mails, in the form of MailItemStruct variables, to our
database. A goroutine is setup to listen on that
channel and handles storage.

Meanwhile this method will loop forever and wait for client connections (blocking).
When a connection is recieved a goroutine is started to create a new MailItemStruct
and parser and the parser process is started. If the parsing is successful
the MailItemStruct is added to a channel. An receivers passed in will be
listening on that channel and may do with the mail item as they wish.
*/
func Dispatcher(serverPool ServerPool, handle *net.TCPListener, receivers []receiver.IMailItemReceiver) {
	/*
	 * Setup our receivers. These guys are basically subscribers to
	 * the MailItem channel.
	 */
	mailItemChannel := make(chan mailitem.MailItem, 1000)

	go func() {
		log.Println("libmailslurper: INFO -", len(receivers), "receiver(s) listening")

		for {
			select {
			case item := <-mailItemChannel:
				for _, r := range receivers {
					go r.Receive(&item)
				}
			}
		}
	}()

	/*
	 * Now start accepting connections for SMTP
	 */
	for {
		connection, err := handle.Accept()
		if err != nil {
			log.Panicf("libmailslurper: ERROR - Error while accepting SMTP requests: %s", err)
		}

		serverPool.NextWorker(connection.(*net.TCPConn), mailItemChannel).Work()
	}
}
