package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"oficina/internal/auth"
	"oficina/internal/platform"
	"oficina/internal/registry"
	"oficina/internal/workorders"
	"oficina/migrations"
	"os"
	"strings"
	"time"
)

func main() {
	if e := run(); e != nil {
		slog.Error("seed failed", "reason", e.Error())
		os.Exit(1)
	}
}
func run() error {
	username, password := strings.TrimSpace(os.Getenv("ADMIN_USERNAME")), os.Getenv("ADMIN_PASSWORD")
	if len(username) < 1 || len(username) > 100 || len(password) < 12 || len(password) > 256 {
		return errors.New("informe ADMIN_USERNAME (1–100 caracteres) e ADMIN_PASSWORD (12–256 bytes)")
	}
	if os.Getenv("DATABASE_URL") == "" {
		return errors.New("DATABASE_URL é obrigatória")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	db, e := platform.Open(ctx, os.Getenv("DATABASE_URL"))
	if e != nil {
		return errors.New("não foi possível conectar ao banco")
	}
	defer db.Close()
	if e = migrations.Apply(ctx, db); e != nil {
		return errors.New("não foi possível aplicar migrações")
	}
	hash, e := auth.PasswordHash(password)
	if e != nil {
		return e
	}
	_, e = db.Exec(ctx, "INSERT INTO administrators(id,username,password_hash) VALUES($1,$2,$3) ON CONFLICT(username) DO NOTHING", uuid.NewString(), username, string(hash))
	if e != nil {
		return errors.New("não foi possível gravar administrador")
	}
	if os.Getenv("DEMO_DATA") == "true" {
		if e = demo(ctx, db); e != nil {
			return e
		}
	}
	fmt.Println("Seed concluído.")
	return nil
}
func demo(ctx context.Context, db *pgxpool.Pool) error {
	var exists bool
	if e := db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM customers WHERE document='52998224725')").Scan(&exists); e != nil {
		return e
	}
	if exists {
		return nil
	}
	r := &registry.Service{DB: db}
	makeRecord := func(kind registry.Kind, payload string) (platform.Record, error) {
		data, e := platform.Decode([]byte(payload))
		if e != nil {
			return nil, e
		}
		return r.Write(ctx, kind, "", data, true)
	}
	customer, e := makeRecord(registry.CustomerKind, `{"name":"Cliente Demonstração","documentType":"CPF","document":"52998224725","email":"cliente@example.com","phone":null}`)
	if e != nil {
		return e
	}
	vehicle, e := makeRecord(registry.VehicleKind, fmt.Sprintf(`{"customerId":%q,"plate":"DEM1A23","brand":"Marca Demo","model":"Hatch de demonstração","year":2024}`, customer["id"]))
	if e != nil {
		return e
	}
	service, e := makeRecord(registry.CatalogServiceKind, `{"name":"Revisão de freios","description":"Serviço sintético para demonstrar o fluxo","priceCents":15000}`)
	if e != nil {
		return e
	}
	part, e := makeRecord(registry.InventoryItemKind, `{"sku":"DEMO-PASTILHA","name":"Pastilha de demonstração","type":"PART","priceCents":8900,"initialQuantity":20}`)
	if e != nil {
		return e
	}
	_, e = (&workorders.Service{DB: db}).Create(ctx, platform.Text(customer, "id"), platform.Text(vehicle, "id"), "Ruído ao frear — demonstração", workorders.Selection{ServiceIDs: []string{platform.Text(service, "id")}, Parts: []workorders.PartSelection{{InventoryItemID: platform.Text(part, "id"), Quantity: 2}}})
	return e
}
