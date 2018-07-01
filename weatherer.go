package weatherer

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/y-yagi/goext/osext"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

var (
	schema = `
CREATE TABLE weathers (
	id integer primary key autoincrement not null,
	area varchar not null,
	date date not null,
	hour integer not null,
	temperature float not null,
	precipitation float,
	wind_speed float,
	wind_direction varchar,
	created_at datetime not null,
	unique(area, date)
);
`
	insertQuery = `
INSERT INTO weathers
	(area , date, hour, temperature, precipitation, wind_speed, wind_direction, created_at)
	VALUES
	($1, $2, $3, $4, $5, $6, $7, $8)
`

	selectQuery = `
SELECT id, date, hour, temperature FROM weathers WHERE date BETWEEN $1 AND $2 ORDER BY date
`
)

// Weatherer is a weatherer module.
type Weatherer struct {
	database string
}

// Weather is type for `weathers` table
type Weather struct {
	ID          int       `db:"id"`
	Date        time.Time `db:"date"`
	Hour        int       `db:"hour"`
	Temperature float64   `db:"temperature"`
}

// NewWeatherer creates a new weatherer.
func NewWeatherer(database string) *Weatherer {
	weatherer := &Weatherer{database: database}
	return weatherer
}

// InitDB initialize database.
func (w *Weatherer) InitDB() error {
	if osext.IsExist(w.database) {
		return nil
	}

	db, err := sqlx.Connect("sqlite3", w.database)
	if err != nil {
		return err
	}
	defer db.Close()

	db.MustExec(schema)

	return nil
}

// Import file to database
func (w *Weatherer) Import(filename string) error {
	headerLines := 5

	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	r := csv.NewReader(transform.NewReader(file, japanese.ShiftJIS.NewDecoder()))
	r.FieldsPerRecord = -1
	if err != nil {
		return err
	}

	records, err := r.ReadAll()
	if err != nil {
		return err
	}

	db, err := sqlx.Connect("sqlite3", w.database)
	if err != nil {
		return err
	}

	format := "2006/1/2 15:04:05"
	loc, _ := time.LoadLocation("Asia/Tokyo")
	_, filename = filepath.Split(filename)
	area := strings.Split(filename, "_")[0]

	tx := db.MustBegin()
	for _, record := range records[headerLines:] {
		t, err := time.ParseInLocation(format, record[0], loc)
		if err != nil {
			return err
		}
		tx.MustExec(insertQuery, area, t, t.Hour(), record[1], record[4], record[7], record[9], time.Now())
	}

	tx.Commit()
	return nil
}

func (w *Weatherer) SelectWeathers(start time.Time, end time.Time) ([]Weather, error) {
	db, err := sqlx.Connect("sqlite3", w.database)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	weathers := []Weather{}
	err = db.Select(&weathers, selectQuery, start, end)
	if err != nil {
		return nil, err
	}

	return weathers, nil
}
