package tableview

import (
	"fmt"
	"html/template"
	"net/http"
	"reflect"

	"gorm.io/gorm"
)

type TableView struct {
	Name       string
	Model      interface{}
	Filters    []FilterField
	Sortable   []string
	ColumnData []Column
}

type FilterField struct {
	Name    string
	Type    string // "dropdown", "checkbox"
	Options []string
}

type Column struct {
	Name  string
	Field string
}

// Template for the table view
const baseTemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>{{.Title}}</title>
</head>
<body>
	<h1>{{.Title}}</h1>
	<form id="filters" method="GET">
		{{range .Filters}}
			<label>{{.Name}}</label>
			{{if eq .Type "dropdown"}}
			<select name="{{.Name}}">
				<option value="">All</option>
				{{range .Options}}
				<option value="{{.}}">{{.}}</option>
				{{end}}
			</select>
			{{end}}
		{{end}}
		<button type="submit">Apply Filters</button>
	</form>
	<table border="1">
		<tr>
			{{range .Headers}}
			<th><a href="?sort={{.}}&{{$.Query}}">{{.}}</a></th>
			{{end}}
		</tr>
		{{range .Rows}}
		<tr>
			{{range .}}
			<td>{{.}}</td>
			{{end}}
		</tr>
		{{end}}
	</table>
</body>
</html>
`

func GenerateTableView(w http.ResponseWriter, r *http.Request, db *gorm.DB, view TableView) {
	// Extract the model type
	modelType := reflect.TypeOf(view.Model).Elem()

	// Dynamically retrieve data
	rows := reflect.New(reflect.SliceOf(reflect.PtrTo(modelType))).Interface()
	query := db.Model(view.Model)

	// Apply filters
	for _, filter := range view.Filters {
		if value := r.URL.Query().Get(filter.Name); value != "" {
			query = query.Where(fmt.Sprintf("%s = ?", filter.Name), value)
		}
	}

	// Apply sorting
	sort := r.URL.Query().Get("sort")
	if sort != "" && contains(view.Sortable, sort) {
		query = query.Order(sort)
	}

	query.Find(rows)

	// Prepare data for the template
	headers := []string{}
	for _, col := range view.ColumnData {
		headers = append(headers, col.Name)
	}

	var data [][]string
	rowsVal := reflect.ValueOf(rows).Elem()
	for i := 0; i < rowsVal.Len(); i++ {
		row := rowsVal.Index(i).Interface()
		rowData := []string{}
		for _, col := range view.ColumnData {
			field := reflect.ValueOf(row).Elem().FieldByName(col.Field)
			rowData = append(rowData, fmt.Sprintf("%v", field.Interface()))
		}
		data = append(data, rowData)
	}

	// Remove existing sort parameter from query
	queryParams := r.URL.Query()
	queryParams.Del("sort")

	tmplData := struct {
		Title   string
		Filters []FilterField
		Headers []string
		Rows    [][]string
		Query   string
	}{
		Title:   view.Name,
		Filters: view.Filters,
		Headers: headers,
		Rows:    data,
		Query:   queryParams.Encode(),
	}

	tmpl, err := template.New("table").Parse(baseTemplate)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse template: %v", err), http.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, tmplData)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to execute template: %v", err), http.StatusInternalServerError)
	}
}

func contains(slice []string, item string) bool {
	for _, elem := range slice {
		if elem == item {
			return true
		}
	}
	return false
}
