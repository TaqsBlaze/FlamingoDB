package main

import (
	"fmt"
	"path/filepath"
	
	"github.com/TaqsBlaze/FlamingoDB"
)

func main() {
	dbPath := filepath.Join(".", "demo.db")
	
	// Connect to engine (creates if not exists, opens if it does)
	db, err := flamingodb.Connect(dbPath)
	if err != nil {
		panic(err)
	}
	defer db.Close()
	
	fmt.Println("Successfully connected to FlamingoDB.")
	
	// Create table
	_, err = db.Run("CREATE TABLE demo_table (id INT, label VARCHAR);")
	if err != nil {
		panic(err)
	}
	fmt.Println("Table created successfully.")
	
	// Insert data
	_, err = db.Run("INSERT INTO demo_table VALUES (42, 'Hello from outside!');")
	if err != nil {
		panic(err)
	}
	fmt.Println("Data inserted successfully.")
	
	// Query data
	res, err := db.Run("SELECT * FROM demo_table;")
	if err != nil {
		panic(err)
	}
	fmt.Println("Query executed successfully. Result:")
	for _, row := range res.Rows {
		fmt.Printf("ID: %v | Label: %v\n", row.Values[0].Int, row.Values[1].Str)
	}
}
