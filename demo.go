package main

import (
	"log"
	"net/http"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"tablow/tableview"
)

type User struct {
	ID   int
	Name string
	Age  int
}

func main() {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to the database: %v", err)
	}

	// Set up a sample table
	db.AutoMigrate(&User{})
	db.Create(&User{Name: "Alice", Age: 30})
	db.Create(&User{Name: "Bob", Age: 25})
	db.Create(&User{Name: "Charlie", Age: 35})

	// Define the table view
	view := tableview.TableView{
		Name:  "Dynamic Table",
		Model: &User{},
		Filters: []tableview.FilterField{
			{Name: "Name", Type: "dropdown", Options: []string{"Alice", "Bob"}},
		},
		Sortable: []string{"ID", "Name", "Age"},
		ColumnData: []tableview.Column{
			{Name: "ID", Field: "ID"},
			{Name: "Name", Field: "Name"},
			{Name: "Age", Field: "Age"},
		},
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tableview.GenerateTableView(w, r, db, view)
	})

	log.Println("Server is running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
