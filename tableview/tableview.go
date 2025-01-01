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
	Editable   bool
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

// Template for just the table component
const baseTemplate = `
<form id="filters-{{.Title}}" method="GET">
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
`

// Template for just the editable table component
const editableTableViewTemplate = `
<form id="filters-{{.Title}}" method="GET">
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
		<th>{{.}}</th>
		{{end}}
		<th>Actions</th>
	</tr>
	{{range $rowIndex, $row := .Rows}}
	<tr id="view-row-{{$rowIndex}}-{{$.Title}}">
		{{range $.Fields}}
		<td>{{index $row .}}</td>
		{{end}}
		<td>
			<button type="button" id="edit-btn-{{$rowIndex}}-{{$.Title}}" onclick="toggleEdit('{{$.Title}}', {{$rowIndex}})">Edit</button>
		</td>
	</tr>
	<tr id="edit-row-{{$rowIndex}}-{{$.Title}}" style="display: none;">
		<form id="edit-form-{{$rowIndex}}-{{$.Title}}" method="POST">
			<input type="hidden" name="id" value="{{index $row "ID"}}">
			{{range $field := $.Fields}}
				<td>
					<input type="text" name="col_{{$field}}" value="{{index $row $field}}">
				</td>
			{{end}}
			<td>
				<button type="submit">Save</button>
				<button type="button" onclick="toggleEdit('{{$.Title}}', {{$rowIndex}})">Cancel</button>
			</td>
		</form>
	</tr>
	{{end}}
</table>

<script>
function toggleEdit(tableName, rowIndex) {
	const viewRow = document.getElementById('view-row-' + rowIndex + '-' + tableName);
	const editRow = document.getElementById('edit-row-' + rowIndex + '-' + tableName);
	const editBtn = document.getElementById('edit-btn-' + rowIndex + '-' + tableName);
	
	if (viewRow.style.display !== 'none') {
		viewRow.style.display = 'none';
		editRow.style.display = 'table-row';
		editBtn.innerText = 'Save';
	} else {
		document.getElementById('edit-form-' + rowIndex + '-' + tableName).submit();
	}
}
</script>
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

func GenerateEditableTableView(w http.ResponseWriter, r *http.Request, db *gorm.DB, view TableView) {
	if r.Method == "POST" {
		// Parse the form data
		if err := r.ParseForm(); err != nil {
			http.Error(w, fmt.Sprintf("Failed to parse form data: %v", err), http.StatusBadRequest)
			return
		}

		// Create a map for the update data
		updateData := make(map[string]interface{})

		// Get the record ID
		id := r.FormValue("id")
		if id == "" {
			http.Error(w, "Missing record ID", http.StatusBadRequest)
			return
		}

		// Extract values using the Field names (not Display names)
		for _, col := range view.ColumnData {
			formValue := r.FormValue("col_" + col.Field)
			if formValue != "" {
				updateData[col.Field] = formValue
			}
		}

		// Log the update data for debugging
		fmt.Printf("Update data: %+v\n", updateData)

		result := db.Model(view.Model).Where("id = ?", id).Updates(updateData)
		if result.Error != nil {
			http.Error(w, "Failed to update record: "+result.Error.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, r.URL.Path, http.StatusSeeOther)
		return
	}

	// Fetch and process the data
	modelType := reflect.TypeOf(view.Model).Elem()
	rows := reflect.New(reflect.SliceOf(reflect.PtrTo(modelType))).Interface()

	result := db.Model(view.Model).Find(rows)
	if result.Error != nil {
		http.Error(w, "Failed to fetch data", http.StatusInternalServerError)
		return
	}

	// Convert the data to a slice of maps for template rendering
	var tableData []map[string]interface{}
	rowsVal := reflect.ValueOf(rows).Elem()
	for i := 0; i < rowsVal.Len(); i++ {
		row := rowsVal.Index(i).Elem()
		rowData := make(map[string]interface{})

		// Always include ID
		if idField := row.FieldByName("ID"); idField.IsValid() {
			rowData["ID"] = idField.Interface()
		}

		// Include all configured columns
		for _, col := range view.ColumnData {
			field := row.FieldByName(col.Field)
			if field.IsValid() {
				rowData[col.Field] = field.Interface()
			}
		}
		tableData = append(tableData, rowData)
	}

	// Create template data structure
	templateData := struct {
		Title   string
		Headers []string
		Fields  []string
		Filters []FilterField
		Rows    []map[string]interface{}
		Query   string
	}{
		Title:   view.Name,
		Headers: extractHeaders(view.ColumnData),
		Fields:  extractFields(view.ColumnData),
		Filters: view.Filters,
		Rows:    tableData,
		Query:   r.URL.Query().Encode(),
	}

	t := template.Must(template.New("table").Parse(editableTableViewTemplate))
	if err := t.Execute(w, templateData); err != nil {
		http.Error(w, fmt.Sprintf("Template execution error: %v", err), http.StatusInternalServerError)
	}
}

// Helper function to extract headers from ColumnData
func extractHeaders(columns []Column) []string {
	headers := make([]string, len(columns))
	for i, col := range columns {
		headers[i] = col.Name
	}
	return headers
}

// Helper function to extract actual field names
func extractFields(columns []Column) []string {
	fields := make([]string, len(columns))
	for i, col := range columns {
		fields[i] = col.Field
	}
	return fields
}

// Helper function to fetch table data (implement if it doesn't exist)
func fetchTableData(db *gorm.DB, view TableView) []map[string]interface{} {
	var results []map[string]interface{}
	db.Model(view.Model).Find(&results)
	return results
}
