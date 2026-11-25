// Package migrations SQL migratsiya fayllarini binar ichiga joylaydi.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
