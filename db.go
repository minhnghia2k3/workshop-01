package main

import (
	"database/sql"
)

func connect() (*sql.DB, error) {
	var err error
	//
	db, err = sql.Open("mysql", "root:Admin123456@tcp(localhost:3306)/workshop_db")
	if err != nil {
		return nil, err
	}

	sqlStmt := `CREATE TABLE IF NOT EXISTS articles (
		id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
		title VARCHAR(255) NOT NULL,
		content TEXT NOT NULL
	);`

	_, err = db.Exec(sqlStmt)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func dbCreateArticle(article *Article) error {
	query, err := db.Prepare("insert into articles(title,content) values (?,?)")
	defer query.Close()

	if err != nil {
		return err
	}
	_, err = query.Exec(article.Title, article.Content)

	if err != nil {
		return err
	}

	return nil
}

func dbGetAllArticles() ([]*Article, error) {
	query, err := db.Prepare("select id, title, content from articles")
	defer query.Close()

	if err != nil {
		return nil, err
	}
	result, err := query.Query()

	if err != nil {
		return nil, err
	}
	articles := make([]*Article, 0)
	for result.Next() {
		data := new(Article)
		err := result.Scan(
			&data.ID,
			&data.Title,
			&data.Content,
		)
		if err != nil {
			return nil, err
		}
		articles = append(articles, data)
	}

	return articles, nil
}

func dbGetArticle(articleID string) (*Article, error) {
	query, err := db.Prepare("select id, title, content from articles where id = ?")
	defer query.Close()

	if err != nil {
		return nil, err
	}
	result := query.QueryRow(articleID)
	data := new(Article)
	err = result.Scan(&data.ID, &data.Title, &data.Content)

	if err != nil {
		return nil, err
	}

	return data, nil
}

func dbUpdateArticle(id string, article *Article) error {
	query, err := db.Prepare("UPDATE articles SET title = ?, content = ? WHERE id = ?")
	if err != nil {
		return err
	}
	defer query.Close()

	_, err = query.Exec(article.Title, article.Content, id)

	if err != nil {
		return err
	}

	return nil
}

func dbDeleteArticle(id string) error {
	query, err := db.Prepare("delete from articles where id=?")
	defer query.Close()

	if err != nil {
		return err
	}
	_, err = query.Exec(id)

	if err != nil {
		return err
	}

	return nil
}
