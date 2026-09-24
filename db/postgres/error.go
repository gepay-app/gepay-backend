package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// IsUniqueViolation mendeteksi unique constraint violation (SQLSTATE 23505).
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// IsForeignKeyViolation mendeteksi foreign key violation (SQLSTATE 23503).
func IsForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

// IsNotNullViolation mendeteksi not null violation (SQLSTATE 23502).
func IsNotNullViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23502"
}

// IsCheckViolation mendeteksi check constraint violation (SQLSTATE 23514).
func IsCheckViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23514"
}

// IsSerializationFailure mendeteksi serialization failure (SQLSTATE 40001).
func IsSerializationFailure(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "40001"
}

// IsDeadlockDetected mendeteksi deadlock detected (SQLSTATE 40P01).
func IsDeadlockDetected(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "40P01"
}

// IsStringDataRightTruncation mendeteksi string data right truncation (SQLSTATE 22001).
func IsStringDataRightTruncation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "22001"
}

// IsNumericValueOutOfRange mendeteksi numeric value out of range (SQLSTATE 22003).
func IsNumericValueOutOfRange(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "22003"
}

// IsInvalidTextRepresentation mendeteksi invalid text representation (SQLSTATE 22P02).
// Contoh: UUID format salah, cast gagal.
func IsInvalidTextRepresentation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "22P02"
}

// IsQueryCanceled mendeteksi query canceled / statement timeout (SQLSTATE 57014).
func IsQueryCanceled(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "57014"
}

// GetErrorCode mengembalikan SQLSTATE code dari error PostgreSQL.
// Mengembalikan string kosong jika bukan error PostgreSQL.
func GetErrorCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}
