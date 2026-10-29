CREATE TABLE IF NOT EXISTS administrators (
 id uuid PRIMARY KEY,
 username text NOT NULL UNIQUE,
 password_hash text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE customers (
 id uuid PRIMARY KEY,
 data jsonb NOT NULL,
 document text GENERATED ALWAYS AS (data->>'document') STORED UNIQUE NOT NULL,
 CHECK (data->>'documentType' IN ('CPF','CNPJ')),
 CHECK (jsonb_typeof(data->'active') = 'boolean')
);
CREATE TABLE vehicles (
 id uuid PRIMARY KEY,
 data jsonb NOT NULL,
 customer_id uuid GENERATED ALWAYS AS ((data->>'customerId')::uuid) STORED REFERENCES customers(id) NOT NULL,
 plate text GENERATED ALWAYS AS (data->>'plate') STORED UNIQUE NOT NULL
);
CREATE TABLE services (
 id uuid PRIMARY KEY,
 data jsonb NOT NULL,
 CHECK ((data->>'priceCents')::bigint BETWEEN 0 AND 9007199254740991)
);
CREATE TABLE inventory_items (
 id uuid PRIMARY KEY,
 data jsonb NOT NULL,
 sku text GENERATED ALWAYS AS (data->>'sku') STORED UNIQUE NOT NULL,
 CHECK ((data->>'priceCents')::bigint BETWEEN 0 AND 9007199254740991),
 CHECK ((data->>'balance')::bigint >= 0),
 CHECK (data->>'type' IN ('PART','SUPPLY'))
);
CREATE SEQUENCE service_order_number;
CREATE TABLE service_orders (
 id uuid PRIMARY KEY,
 data jsonb NOT NULL,
 number bigint GENERATED ALWAYS AS ((data->>'number')::bigint) STORED UNIQUE NOT NULL,
 customer_id uuid GENERATED ALWAYS AS ((data->>'customerId')::uuid) STORED REFERENCES customers(id) NOT NULL,
 vehicle_id uuid GENERATED ALWAYS AS ((data->>'vehicleId')::uuid) STORED REFERENCES vehicles(id) NOT NULL,
 status text GENERATED ALWAYS AS (data->>'status') STORED NOT NULL,
 version bigint GENERATED ALWAYS AS ((data->>'version')::bigint) STORED NOT NULL,
 token_hash bytea UNIQUE,
 token_expires_at timestamptz,
 CHECK (version > 0),
 CHECK (status IN ('RECEIVED','IN_DIAGNOSIS','AWAITING_APPROVAL','IN_EXECUTION','COMPLETED','DELIVERED'))
);
CREATE INDEX service_orders_customer_idx ON service_orders(customer_id);
CREATE INDEX service_orders_vehicle_idx ON service_orders(vehicle_id);
CREATE INDEX service_orders_status_idx ON service_orders(status);
CREATE INDEX vehicles_customer_idx ON vehicles(customer_id);
CREATE TABLE inventory_movements (
 id uuid PRIMARY KEY,
 inventory_item_id uuid NOT NULL REFERENCES inventory_items(id),
 service_order_id uuid REFERENCES service_orders(id),
 data jsonb NOT NULL,
 CHECK ((data->>'quantityDelta')::bigint <> 0)
);
CREATE INDEX inventory_movements_item_idx ON inventory_movements(inventory_item_id);
CREATE UNIQUE INDEX consumption_once ON inventory_movements(inventory_item_id,service_order_id) WHERE service_order_id IS NOT NULL;
