package domain

// OrderStatus is the lifecycle state of an order. The only legal flow is
// angefragt -> bestätigt -> in Arbeit -> fertig -> abgeholt.
type OrderStatus string

const (
	StatusAngefragt  OrderStatus = "angefragt"
	StatusBestaetigt OrderStatus = "bestätigt"
	StatusInArbeit   OrderStatus = "in Arbeit"
	StatusFertig     OrderStatus = "fertig"
	StatusAbgeholt   OrderStatus = "abgeholt"
)

// OrderStatusOrder is the canonical order of the lifecycle. Consumers use it
// to decide which transitions are legal.
var OrderStatusOrder = []OrderStatus{
	StatusAngefragt,
	StatusBestaetigt,
	StatusInArbeit,
	StatusFertig,
	StatusAbgeholt,
}

// Customer is the customer referenced by an order.
type Customer struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

// Vehicle is the vehicle referenced by an order.
type Vehicle struct {
	LicensePlate string `json:"license_plate"`
	Brand        string `json:"brand"`
	Model        string `json:"model"`
	Mileage      int    `json:"mileage"`
}

// OrderItem is one position of an order. hours/quantity/unit_price_cents are
// optional depending on the kind ("labor" uses hours, "part" uses quantity).
type OrderItem struct {
	ID             int      `json:"id"`
	Kind           string   `json:"kind"`
	Description    string   `json:"description"`
	Hours          *float64 `json:"hours,omitempty"`
	Quantity       *int     `json:"quantity,omitempty"`
	UnitPriceCents *int     `json:"unit_price_cents,omitempty"`
	TotalCents     int      `json:"total_cents"`
}

// StatusEvent is one entry of an order's status history.
type StatusEvent struct {
	FromStatus *OrderStatus `json:"from_status"`
	ToStatus   OrderStatus  `json:"to_status"`
	ChangedBy  string       `json:"changed_by"`
	ChangedAt  string       `json:"changed_at"`
}

// OrderDetail is the shared detailed representation of an order.
type OrderDetail struct {
	OrderNumber        string        `json:"order_number"`
	Status             OrderStatus   `json:"status"`
	RequestedDate      string        `json:"requested_date"`
	ProblemDescription string        `json:"problem_description"`
	CreatedAt          string        `json:"created_at"`
	Customer           Customer      `json:"customer"`
	Vehicle            Vehicle       `json:"vehicle"`
	Items              []OrderItem   `json:"items"`
	History            []StatusEvent `json:"history"`
}

// InvoiceLine is one line of an invoice. Kind is "labor" or "part" and drives
// the "Position" column of the customer invoice table.
type InvoiceLine struct {
	Kind           string `json:"kind"`
	Description    string `json:"description"`
	Quantity       int    `json:"quantity"`
	UnitPriceCents int    `json:"unit_price_cents"`
	TotalCents     int    `json:"total_cents"`
}

// Invoice is the invoice calculated by the worker for a completed order.
type Invoice struct {
	InvoiceNumber string        `json:"invoice_number"`
	OrderNumber   string        `json:"order_number"`
	IssuedAt      string        `json:"issued_at"`
	Lines         []InvoiceLine `json:"lines"`
	LaborCents    int           `json:"labor_cents"`
	PartsCents    int           `json:"parts_cents"`
	NetCents      int           `json:"net_cents"`
	VatCents      int           `json:"vat_cents"`
	GrossCents    int           `json:"gross_cents"`
}

// Employee is the workshop account attached to a session.
type Employee struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// CreateAppointmentRequest is the body of POST /api/customer/appointments.
type CreateAppointmentRequest struct {
	Customer           Customer `json:"customer"`
	Vehicle            Vehicle  `json:"vehicle"`
	RequestedDate      string   `json:"requested_date"`
	ProblemDescription string   `json:"problem_description"`
}

// LoginRequest is the body of POST /api/workshop/login.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse is the successful answer of POST /api/workshop/login.
type LoginResponse struct {
	Token    string   `json:"token"`
	Employee Employee `json:"employee"`
}

// OrderListResponse is the body of GET /api/workshop/orders.
type OrderListResponse struct {
	Orders []OrderDetail `json:"orders"`
}

// UpdateStatusRequest is the body of POST /api/workshop/orders/{order_number}/status.
type UpdateStatusRequest struct {
	Status OrderStatus `json:"status"`
}

// ItemRequest is the shared body of creating and updating an order item.
// kind is either "labor" or "part".
type ItemRequest struct {
	Kind           string   `json:"kind"`
	Description    string   `json:"description"`
	Hours          *float64 `json:"hours,omitempty"`
	Quantity       *int     `json:"quantity,omitempty"`
	UnitPriceCents *int     `json:"unit_price_cents,omitempty"`
}

// DashboardResponse is the body of GET /api/workshop/dashboard.
type DashboardResponse struct {
	OpenOrders        int `json:"open_orders"`
	CompletedToday    int `json:"completed_today"`
	RevenueMonthCents int `json:"revenue_month_cents"`
}

// HealthResponse is the body of GET /api/health.
type HealthResponse struct {
	Status string `json:"status"`
}

// ErrorBody is the uniform error payload of every failed request.
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail is the code and human message inside an ErrorBody. It never
// carries internals such as SQL fragments, stacktraces or file paths.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	// Details carries one entry per rejected input field on a validation_error.
	// It is omitted for every other error, so the uniform error body keeps its
	// {"error":{"code","message"}} shape everywhere else.
	Details []FieldError `json:"details,omitempty"`
}

// FieldError names one rejected input field of a request and why it was
// rejected. It is safe to expose: it names the contract field, never an
// internal detail.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}
