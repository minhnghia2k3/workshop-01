package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/go-sql-driver/mysql"
)

var router *chi.Mux
var db *sql.DB

type Article struct {
	ID      int           `json:"id"`
	Title   string        `json:"title"`
	Content template.HTML `json:"content"`
}

func catch(err error) {
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
}

func main() {
	var err error

	router = chi.NewRouter()
	router.Use(middleware.Recoverer)

	db, err = connect()
	catch(err)

	router.Use(ChangeMethod)
	router.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	router.Get("/", GetAllArticles)
	router.Post("/upload", UploadHandler)
	router.Route("/articles", func(r chi.Router) {
		r.Get("/", NewArticle)
		r.Post("/", CreateArticle)
		r.Route("/{articleID}", func(r chi.Router) {
			r.Use(ArticleCtx)
			r.Get("/", GetArticle)       // GET /articles/1234
			r.Put("/", UpdateArticle)    // PUT /articles/1234
			r.Delete("/", DeleteArticle) // DELETE /articles/1234
			r.Get("/edit", EditArticle)  // GET /articles/1234/edit
		})
	})

	fmt.Println("The server is running on :3000")
	err = http.ListenAndServe(":3000", router)
	catch(err)
}

func UploadHandler(w http.ResponseWriter, r *http.Request) {
	const MAX_UPLOAD_SIZE = 10 << 20 // 10 MB
	r.Body = http.MaxBytesReader(w, r.Body, MAX_UPLOAD_SIZE)
	if err := r.ParseMultipartForm(MAX_UPLOAD_SIZE); err != nil {
		http.Error(w, "The uploaded file is too big. Please choose a file that's less than 10MB in size", http.StatusBadRequest)
		return
	}

	// Retrieve file from the request
	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Read file content into memory
	fileBuffer := bytes.NewBuffer(nil)
	_, err = io.Copy(fileBuffer, file)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	// Generate a unique file name
	fileName := fmt.Sprintf("%d%s", time.Now().UnixNano(), filepath.Ext(fileHeader.Filename))

	// Configure AWS S3 session
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(os.Getenv("AWS_REGION")),
	})
	if err != nil {
		http.Error(w, "Failed to connect to AWS", http.StatusInternalServerError)
		return
	}

	svc := s3.New(sess)

	// Define the S3 bucket and key
	bucketName := os.Getenv("S3_BUCKET_NAME")
	objectKey := fmt.Sprintf("uploads/%s", fileName)

	// Upload file to S3
	_, err = svc.PutObject(&s3.PutObjectInput{
		Bucket:        aws.String(bucketName),
		Key:           aws.String(objectKey),
		Body:          bytes.NewReader(fileBuffer.Bytes()),
		ContentLength: aws.Int64(int64(fileBuffer.Len())),
		ContentType:   aws.String(fileHeader.Header.Get("Content-Type")),
		ACL:           aws.String("public-read"),
	})
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Failed to upload file to S3", http.StatusInternalServerError)
		return
	}

	// Construct the public URL for the uploaded file
	fileURL := fmt.Sprintf("https://%s.s3.amazonaws.com/%s", bucketName, objectKey)

	// Respond with the file URL
	response, _ := json.Marshal(map[string]string{"location": fileURL})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(response)
}

func ChangeMethod(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			switch method := r.PostFormValue("_method"); method {
			case http.MethodPut:
				fallthrough
			case http.MethodPatch:
				fallthrough
			case http.MethodDelete:
				r.Method = method
			default:
			}
		}
		next.ServeHTTP(w, r)
	})
}

func ArticleCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		articleID := chi.URLParam(r, "articleID")
		article, err := dbGetArticle(articleID)
		if err != nil {
			fmt.Println(err)
			http.Error(w, http.StatusText(404), 404)
			return
		}
		ctx := context.WithValue(r.Context(), "article", article)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetAllArticles(w http.ResponseWriter, r *http.Request) {
	articles, err := dbGetAllArticles()
	catch(err)

	t, _ := template.ParseFiles("templates/base.html", "templates/index.html")
	err = t.Execute(w, articles)
	catch(err)
}

func NewArticle(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles("templates/base.html", "templates/new.html")
	err := t.Execute(w, nil)
	catch(err)
}
func CreateArticle(w http.ResponseWriter, r *http.Request) {
	title := r.FormValue("title")
	content := r.FormValue("content")
	article := &Article{
		Title:   title,
		Content: template.HTML(content),
	}

	err := dbCreateArticle(article)
	catch(err)
	http.Redirect(w, r, "/", http.StatusFound)
}

func GetArticle(w http.ResponseWriter, r *http.Request) {
	article := r.Context().Value("article").(*Article)
	t, _ := template.ParseFiles("templates/base.html", "templates/article.html")
	err := t.Execute(w, article)
	catch(err)
}

func EditArticle(w http.ResponseWriter, r *http.Request) {
	article := r.Context().Value("article").(*Article)

	t, _ := template.ParseFiles("templates/base.html", "templates/edit.html")
	err := t.Execute(w, article)
	catch(err)
}

func UpdateArticle(w http.ResponseWriter, r *http.Request) {
	article := r.Context().Value("article").(*Article)

	title := r.FormValue("title")
	content := r.FormValue("content")
	newArticle := &Article{
		Title:   title,
		Content: template.HTML(content),
	}

	err := dbUpdateArticle(strconv.Itoa(article.ID), newArticle)
	catch(err)
	http.Redirect(w, r, fmt.Sprintf("/articles/%d", article.ID), http.StatusFound)
}

func DeleteArticle(w http.ResponseWriter, r *http.Request) {
	article := r.Context().Value("article").(*Article)
	err := dbDeleteArticle(strconv.Itoa(article.ID))
	catch(err)

	http.Redirect(w, r, "/", http.StatusFound)
}
