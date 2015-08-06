// Copyright 2013-2014 Adam Presley. All rights reserved
// Use of this source code is governed by the MIT license
// that can be found in the LICENSE file.

package storage

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/adampresley/golangdb"
	"github.com/mailslurper/libmailslurper/model/attachment"
	"github.com/mailslurper/libmailslurper/model/mailitem"
	"github.com/mailslurper/libmailslurper/sanitization"
)

/*
Creates a global connection handle in a map named "lib".
*/
func ConnectToStorage(connectionInfo *golangdb.DatabaseConnection) error {
	var err error

	log.Println("Connecting to database", connectionInfo.Database)

	err = connectionInfo.Connect("lib")
	if err != nil {
		return err
	}

	switch connectionInfo.Engine {
	case golangdb.SQLITE:
		CreateSqlliteDatabase()
	}

	return nil
}

/*
Disconnects from the database storage
*/
func DisconnectFromStorage() {
	golangdb.Db["lib"].Close()
}

/*
Returns an attachment by ID
*/
func GetAttachment(mailId, attachmentId string) (attachment.Attachment, error) {
	result := attachment.Attachment{}

	rows, err := golangdb.Db["lib"].Query(`
		SELECT
			  fileName TEXT
			, contentType TEXT
			, content TEXT
		FROM attachment
		WHERE
			id=?
			AND mailItemId=?
	`, attachmentId, mailId)

	if err != nil {
		return result, fmt.Errorf("Error running query to get attachment")
	}

	defer rows.Close()
	rows.Next()

	var fileName string
	var contentType string
	var content string

	rows.Scan(&fileName, &contentType, &content)

	result.Headers = &attachment.AttachmentHeader{
		FileName:    fileName,
		ContentType: contentType,
	}

	result.Contents = content
	return result, nil
}

func getMailQuery(whereClause string) string {
	sql := `
		SELECT
			  mailitem.id AS mailItemId
			, mailitem.dateSent
			, mailitem.fromAddress
			, mailitem.toAddressList
			, mailitem.subject
			, mailitem.xmailer
			, mailitem.body
			, mailitem.contentType
			, mailitem.boundary

		FROM mailitem

		WHERE 1=1 `

	sql = sql + whereClause
	sql = sql + ` ORDER BY mailitem.dateSent DESC `

	return sql
}

/*
Returns a single mail item by ID.
*/
func GetMail(id string) (mailitem.MailItem, error) {
	result := mailitem.MailItem{}
	xssService := sanitization.NewXSSService()

	sql := getMailQuery(" AND mailitem.id=? ")
	rows, err := golangdb.Db["lib"].Query(sql, id)

	if err != nil {
		return result, fmt.Errorf("Error running query to get mail items: %s", err)
	}

	rows.Next()

	var mailItemId string
	var dateSent string
	var fromAddress string
	var toAddressList string
	var subject string
	var xmailer string
	var body string
	var contentType string
	var boundary string

	rows.Scan(&mailItemId, &dateSent, &fromAddress, &toAddressList, &subject, &xmailer, &body, &contentType, &boundary)

	result = mailitem.MailItem{
		Id:          mailItemId,
		DateSent:    dateSent,
		FromAddress: fromAddress,
		ToAddresses: strings.Split(toAddressList, "; "),
		Subject:     xssService.SanitizeString(subject),
		XMailer:     xssService.SanitizeString(xmailer),
		Body:        xssService.SanitizeString(body),
		ContentType: contentType,
		Boundary:    boundary,
	}

	/*
	 * Get attachments
	 */
	sql = `
		SELECT
			  attachment.id AS attachmentId
			, attachment.fileName
			, attachment.contentType

		FROM attachment
		WHERE
			attachment.mailItemId=?`

	attachmentRows, err := golangdb.Db["lib"].Query(sql, mailItemId)
	if err != nil {
		return result, err
	}

	attachments := make([]*attachment.Attachment, 0)

	for attachmentRows.Next() {
		var attachmentId string
		var fileName string
		var contentType string

		attachmentRows.Scan(&attachmentId, &fileName, &contentType)

		newAttachment := &attachment.Attachment{
			Id:     attachmentId,
			MailId: mailItemId,
			Headers: &attachment.AttachmentHeader{
				FileName:    xssService.SanitizeString(fileName),
				ContentType: contentType,
			},
		}

		attachments = append(attachments, newAttachment)
	}

	attachmentRows.Close()

	result.Attachments = attachments

	rows.Close()
	return result, nil
}

/*
Retrieves all stored mail items as an array of MailItem items. Only
returns rows starting at offset and gets up to length records. NOTE:
This code stinks. It gets ALL rows, then returns a slice and the total number
of mail items. Ick!
*/
func GetMailCollection(offset, length int) ([]mailitem.MailItem, int, error) {
	result := make([]mailitem.MailItem, 0)
	xssService := sanitization.NewXSSService()

	sql := getMailQuery("")
	rows, err := golangdb.Db["lib"].Query(sql)

	if err != nil {
		return result, 0, fmt.Errorf("Error running query to get mail items: %s", err)
	}

	/*
	 * Loop over our records and grab attachments on the way.
	 */
	for rows.Next() {
		var mailItemId string
		var dateSent string
		var fromAddress string
		var toAddressList string
		var subject string
		var xmailer string
		var body string
		var contentType string
		var boundary string

		rows.Scan(&mailItemId, &dateSent, &fromAddress, &toAddressList, &subject, &xmailer, &body, &contentType, &boundary)

		newItem := mailitem.MailItem{
			Id:          mailItemId,
			DateSent:    dateSent,
			FromAddress: fromAddress,
			ToAddresses: strings.Split(toAddressList, "; "),
			Subject:     xssService.SanitizeString(subject),
			XMailer:     xssService.SanitizeString(xmailer),
			Body:        xssService.SanitizeString(body),
			ContentType: contentType,
			Boundary:    boundary,
		}

		/*
		 * Get attachments
		 */
		sql = `
			SELECT
				  attachment.id AS attachmentId
				, attachment.fileName
			FROM attachment
			WHERE
				attachment.mailItemId=?`

		attachmentRows, err := golangdb.Db["lib"].Query(sql, mailItemId)
		if err != nil {
			return result, 0, err
		}

		attachments := make([]*attachment.Attachment, 0)

		for attachmentRows.Next() {
			var attachmentId string
			var fileName string

			attachmentRows.Scan(&attachmentId, &fileName)

			newAttachment := &attachment.Attachment{
				Id:      attachmentId,
				MailId:  mailItemId,
				Headers: &attachment.AttachmentHeader{FileName: xssService.SanitizeString(fileName)},
			}

			attachments = append(attachments, newAttachment)
		}

		attachmentRows.Close()

		newItem.Attachments = attachments
		result = append(result, newItem)
	}

	rows.Close()

	totalRecords := len(result)
	return result, totalRecords, nil
}

func storeAttachments(mailItemId string, transaction *sql.Tx, attachments []*attachment.Attachment) error {
	for _, a := range attachments {
		attachmentId, err := mailitem.GenerateId()
		if err != nil {
			return fmt.Errorf("Error generating ID for attachment: %s", err)
		}

		statement, err := transaction.Prepare(`
			INSERT INTO attachment (
				  id
				, mailItemId
				, fileName
				, contentType
				, content
			) VALUES (
				  ?
				, ?
				, ?
				, ?
				, ?
			)
		`)

		if err != nil {
			return fmt.Errorf("Error preparing insert attachment statement: %s", err)
		}

		_, err = statement.Exec(
			attachmentId,
			mailItemId,
			a.Headers.FileName,
			a.Headers.ContentType,
			a.Contents,
		)

		if err != nil {
			return fmt.Errorf("Error executing insert attachment in StoreMail: %s", err)
		}

		statement.Close()
		a.Id = attachmentId
	}

	return nil
}

func StoreMail(mailItem *mailitem.MailItem) (string, error) {
	/*
	 * Create a transaction and insert the new mail item
	 */
	transaction, err := golangdb.Db["lib"].Begin()
	if err != nil {
		return "", fmt.Errorf("Error starting transaction in StoreMail: %s", err)
	}

	/*
	 * Insert the mail item
	 */
	statement, err := transaction.Prepare(`
			INSERT INTO mailitem (
				  id
				, dateSent
				, fromAddress
				, toAddressList
				, subject
				, xmailer
				, body
				, contentType
				, boundary
			) VALUES (
				  ?
				, ?
				, ?
				, ?
				, ?
				, ?
				, ?
				, ?
				, ?
			)
		`)

	if err != nil {
		return "", fmt.Errorf("Error preparing insert statement for mail item in StoreMail: %s", err)
	}

	_, err = statement.Exec(
		mailItem.Id,
		mailItem.DateSent,
		mailItem.FromAddress,
		strings.Join(mailItem.ToAddresses, "; "),
		mailItem.Subject,
		mailItem.XMailer,
		mailItem.Body,
		mailItem.ContentType,
		mailItem.Boundary,
	)

	if err != nil {
		transaction.Rollback()
		return "", fmt.Errorf("Error executing insert for mail item in StoreMail: %s", err)
	}

	statement.Close()

	/*
	 * Insert attachments
	 */
	if err = storeAttachments(mailItem.Id, transaction, mailItem.Attachments); err != nil {
		transaction.Rollback()
		return "", fmt.Errorf("Unable to insert attachments in StoreMail: %s", err)
	}

	transaction.Commit()
	log.Printf("New mail item written to database.\n\n")

	return mailItem.Id, nil
}
