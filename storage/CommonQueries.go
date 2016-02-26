package storage

import (
	"strings"
	"time"

	"github.com/mailslurper/libmailslurper/model/search"
)

func getMailAndAttachmentsQuery(whereClause string) string {
	sqlQuery := `
		SELECT
			  mailitem.dateSent
			, mailitem.fromAddress
			, mailitem.toAddressList
			, mailitem.subject
			, mailitem.xmailer
			, mailitem.body
			, mailitem.contentType
			, mailitem.boundary
			, attachment.id AS attachmentID
			, attachment.fileName
			, attachment.contentType

		FROM mailitem
			LEFT JOIN attachment ON attachment.mailItemID=mailitem.id

		WHERE 1=1 `

	sqlQuery = sqlQuery + whereClause
	sqlQuery = sqlQuery + ` ORDER BY mailitem.dateSent DESC `

	return sqlQuery
}

func getMailCountQuery(mailSearch *search.MailSearch) (string, []interface{}) {
	sqlQuery := `
		SELECT COUNT(id) AS mailItemCount FROM mailitem WHERE 1=1
	`

	var parameters []interface{}
	return addSearchCriteria(sqlQuery, parameters, mailSearch)
}

func getDeleteMailQuery(startDate string) string {
	where := ""

	if len(startDate) > 0 {
		where = where + " AND dateSent <= ? "
	}

	sqlQuery := "DELETE FROM mailitem WHERE 1=1" + where
	return sqlQuery
}

func getInsertMailQuery() string {
	sqlQuery := `
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
	`

	return sqlQuery
}

func getInsertAttachmentQuery() string {
	sqlQuery := `
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
	`

	return sqlQuery
}

func addSearchCriteria(sqlQuery string, parameters []interface{}, mailSearch *search.MailSearch) (string, []interface{}) {
	var date time.Time
	var err error

	if len(strings.TrimSpace(mailSearch.Message)) > 0 {
		sqlQuery += `
			AND (
				mailitem.body LIKE '%?%'
				OR mailitem.subject LIKE '%?%'
			)
		`

		parameters = append(parameters, mailSearch.Message)
	}

	if len(strings.TrimSpace(mailSearch.From)) > 0 {
		sqlQuery += `
			AND mailitem.subject LIKE '%?%'
		`

		parameters = append(parameters, mailSearch.From)
	}

	if len(strings.TrimSpace(mailSearch.To)) > 0 {
		sqlQuery += `
			AND mailitem.toAddressList LIKE '%?%'
		`

		parameters = append(parameters, mailSearch.To)
	}

	if len(strings.TrimSpace(mailSearch.Start)) > 0 {
		if date, err = time.Parse("2006-01-02", mailSearch.Start); err == nil {
			sqlQuery += `
				AND mailitem.dateSent >= ?
			`

			parameters = append(parameters, date)
		}
	}

	if len(strings.TrimSpace(mailSearch.End)) > 0 {
		if date, err = time.Parse("2006-01-02", mailSearch.End); err == nil {
			sqlQuery += `
				AND mailitem.dateSent <= ?
			`

			parameters = append(parameters, date)
		}
	}

	return sqlQuery, parameters
}
