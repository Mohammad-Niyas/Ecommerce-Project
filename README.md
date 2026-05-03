# VogueLuxe — E-Commerce Platform (Monolithic Architecture)

> **⚠️ Architecture Note:** This is the **monolithic** version of VogueLuxe. All business domains — authentication, catalog, orders, payments, inventory, and wallet — are bundled into a single deployable Go binary. A microservices decomposition is planned as a separate initiative.

A production-grade e-commerce backend built with **Go** and the **Gin** web framework, following the **MVC (Model-View-Controller)** pattern. The application ships as a single process backed by PostgreSQL, with server-side rendered HTML templates and third-party integrations for payments, media, and communications.

---

## Why Monolithic?

This architecture was chosen deliberately for the initial version:

| Aspect | Rationale |
| :--- | :--- |
| **Single deployment unit** | One Go binary, one `Dockerfile`, one K8s Deployment — minimal operational overhead |
| **Shared database** | All 22 domain models live in a single PostgreSQL database with foreign key relationships managed by GORM |
| **In-process communication** | Controllers call each other directly; no network hops, no serialization overhead |
| **Single request lifecycle** | One Gin router handles all ~100 routes for both Admin and User panels |
| **Simpler debugging** | Full stack traces in one process, structured logging via Uber-Zap |

### Monolithic Boundaries (Future Microservice Candidates)

```
┌──────────────────────────────────────────────────────────────────┐
│                     SINGLE GO BINARY (:8080)                     │
│                                                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐   │
│  │  Auth & User  │  │   Catalog    │  │   Orders & Payments  │   │
│  │  Management   │  │   (Products, │  │   (Cart, Checkout,   │   │
│  │  (Signup,     │  │   Categories,│  │   Razorpay, Invoice) │   │
│  │   Login, OTP, │  │   Variants,  │  │                      │   │
│  │   OAuth, JWT) │  │   Offers)    │  │                      │   │
│  └──────┬───────┘  └──────┬───────┘  └──────────┬───────────┘   │
│         │                 │                      │               │
│  ┌──────┴───────┐  ┌──────┴───────┐  ┌──────────┴───────────┐   │
│  │   Wallet &   │  │   Coupons &  │  │   Admin Dashboard    │   │
│  │   Wishlist   │  │   Promotions │  │   (Reports, PDF,     │   │
│  │              │  │              │  │    User Mgmt, Excel) │   │
│  └──────────────┘  └──────────────┘  └──────────────────────┘   │
│                                                                  │
│                    ┌──────────────────┐                          │
│                    │   PostgreSQL     │                          │
│                    │   (22 tables)    │                          │
│                    └──────────────────┘                          │
└──────────────────────────────────────────────────────────────────┘
```

---

## Tech Stack

| Layer | Technology | Purpose |
| :--- | :--- | :--- |
| **Language** | Go 1.24 | High-performance, compiled backend |
| **Web Framework** | Gin-Gonic v1.11 | HTTP routing, middleware, SSR template rendering |
| **ORM** | GORM v1.31 | PostgreSQL interactions, AutoMigrate schema sync |
| **Database** | PostgreSQL | Relational storage for all 22 domain models |
| **Auth** | JWT (HS256) + Google OAuth 2.0 | Stateless sessions, social login |
| **Payments** | Razorpay API | Checkout, payment verification, retry flows |
| **Media Storage** | Cloudinary | Product image upload, transformation, CDN delivery |
| **Email/OTP** | Brevo (Sendinblue) | Transactional emails, OTP verification |
| **Logging** | Uber-Zap | Structured, leveled logging (JSON in prod) |
| **PDF Generation** | gofpdf | Invoice downloads, sales report exports |
| **Frontend** | Go `html/template` + Tailwind CSS | Server-side rendered admin & user panels |
| **Containerization** | Docker (multi-stage build) | Alpine-based production image |
| **Orchestration** | Kubernetes | Deployment manifests with 2 replicas |

---

## Project Structure

```
VogueLuxe-Ecommerce/
│
├── main.go                          # Application entry point — initializes logger,
│                                    # loads env, connects DB, registers all routes,
│                                    # starts Gin on :8080
│
├── config/
│   ├── dbConnection.go              # PostgreSQL connection + AutoMigrate (22 models)
│   ├── envLoad.go                   # godotenv loader
│   └── googleAuth.go               # Google OAuth2 client configuration
│
├── controllers/
│   ├── AdminController.go           # Admin auth, dashboard, reports (PDF/Excel)
│   ├── AdminOrderController.go      # Order status management, return approvals
│   ├── CategoryController.go        # Category CRUD, category-level offers
│   ├── CouponController.go          # Coupon CRUD, toggle, validation
│   ├── ProductController.go         # Product/variant CRUD, image upload, offers
│   ├── UserAddtoCart.go             # Cart, checkout, Razorpay integration, orders
│   ├── UserController.go           # User auth, profile, address, OAuth, OTP
│   └── WalletController.go         # Wallet top-up, transaction history
│
├── models/
│   ├── UserModels.go                # User, UserDetails
│   ├── AdminModels.go               # Admin
│   ├── Category.go                  # Category, CategoryOffer
│   ├── Product.go                   # Product, ProductImage, ProductVariant, ProductOffer
│   ├── Cart.go                      # Cart, CartItem
│   ├── Order.go                     # Order, OrderItem, ReturnRequest, ShippingAddress
│   ├── Payment.go                   # PaymentDetails
│   ├── Coupon.go                    # Coupon
│   ├── Wallet.go                    # Wallet, WalletTransaction
│   ├── Wishlist.go                  # Wishlist, WishlistItem
│   ├── Address.go                   # Address
│   └── Otp.go                       # OTP
│
├── middleware/
│   └── jwt.go                       # JWT generation/validation, RBAC (Admin/User),
│                                    # no-cache headers, auth redirect
│
├── routers/
│   ├── Admin_Router.go              # ~50 admin routes (dashboard, products, orders, etc.)
│   └── User_Router.go              # ~50 user routes (auth, cart, checkout, profile, etc.)
│
├── utils/
│   ├── Cloudinary.go                # Image upload/delete via Cloudinary SDK
│   ├── Otp.go                       # OTP generation & email dispatch via Brevo API
│   └── paymentRazorpay.go          # Razorpay order creation & signature verification
│
├── pkg/
│   └── logger/
│       └── logger.go                # Uber-Zap logger initialization
│
├── views/
│   ├── Admin/                       # 17 SSR templates (dashboard, product mgmt, etc.)
│   └── User/                        # 23 SSR templates (home, cart, checkout, profile, etc.)
│
├── k8s/
│   ├── deployment.yaml              # 2-replica deployment, env from K8s Secret
│   └── service.yaml                 # Service exposure config
│
├── Dockerfile                       # Multi-stage: golang:1.24-alpine → alpine:latest
├── makefile                         # `make tidy` and `make run` shortcuts
├── go.mod / go.sum                  # Go module dependencies
└── .gitignore / .dockerignore
```

---

## Domain Models (22 Tables)

All models reside in a single PostgreSQL database, auto-migrated via GORM on startup:

```
User ──────┬── UserDetails
           ├── Address
           ├── Cart ──── CartItem ──── ProductVariant
           ├── Wishlist ── WishlistItem
           ├── Wallet ──── WalletTransaction
           └── Order ──┬── OrderItem
                       ├── ReturnRequest
                       ├── ShippingAddress
                       └── PaymentDetails

Category ──── CategoryOffer
Product ──┬── ProductImage
          ├── ProductVariant
          └── ProductOffer

Admin
Otp
Coupon
```

---

## Core Features

### 🔐 Authentication & Authorization
- Email/password signup with OTP verification (Brevo API)
- Google OAuth 2.0 social login
- JWT-based stateless sessions (HS256, 2-hour expiry)
- Role-based access control: `Admin` and `User` middleware guards
- Forgot password flow with OTP reset

### 🛒 Shopping Experience
- Product browsing with dynamic filtering and search
- Multi-variant product support (size, color, stock per variant)
- Wishlist with move-to-cart functionality
- Cart management with real-time stock validation
- Multi-step checkout: Address → Payment Method → Confirm

### 💳 Payments
- Razorpay integration with order creation and signature verification
- Cash on Delivery option
- Wallet-based payments
- Failed payment retry flow
- Pre-order creation for payment validation

### 📦 Order Management
- Full order lifecycle: Placed → Confirmed → Shipped → Delivered
- Individual item cancellation within an order
- Return request submission and admin approval/rejection
- PDF invoice generation and download (gofpdf)
- Order history with detailed item-level view

### 🎟️ Promotions & Coupons
- Coupon CRUD with validity period and usage limits
- Product-level and category-level offers with percentage discounts
- Discount stacking and calculation engine

### 💰 Wallet System
- In-app wallet with top-up via Razorpay
- Wallet balance used during checkout
- Refunds credited to wallet on cancellation/return
- Full transaction history for users and admin

### 🛠️ Admin Panel
- Dashboard with sales analytics and revenue data
- Sales report export: PDF and Excel downloads
- Product management with multi-image upload (Cloudinary)
- User management: block/unblock users
- Category management with nested offers
- Order status updates and return processing
- Coupon management with toggle activation
- Wallet oversight and transaction audit

---

## Getting Started

### Prerequisites
- **Go 1.24+**
- **PostgreSQL** (local or cloud — RDS, Neon, Supabase, etc.)
- **Docker** (optional, for containerized deployment)

### Environment Variables

Create a `.env` file in the project root:

```env
# Database
DB_HOST=localhost
DB_USER=postgres
DB_PASSWORD=your_password
DB_NAME=vogueluxe
DB_PORT=5432

# Auth
SECRETKEY=your_jwt_secret_key
GOOGLE_CLIENT_ID=your_google_client_id
GOOGLE_CLIENT_SECRET=your_google_client_secret

# External Services
CLOUDINARY_CLOUD_NAME=your_cloud_name
CLOUDINARY_API_KEY=your_api_key
CLOUDINARY_API_SECRET=your_api_secret
BREVO_API_KEY=your_brevo_api_key
RAZORPAY_KEY_ID=your_razorpay_key
RAZORPAY_KEY_SECRET=your_razorpay_secret
```

### Run Locally

```bash
# Install dependencies
go mod tidy

# Start the server
go run main.go
# or
make run
```

The application starts on **http://localhost:8080**

### Docker

```bash
# Build image
docker build -t vogueluxe .

# Run container
docker run -p 8080:8080 --env-file .env vogueluxe
```

### Kubernetes

```bash
# Create secrets from env vars
kubectl create secret generic ecommerce-secret --from-env-file=.env

# Deploy
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml
```

---

## API Routes Overview

### Public Routes
| Method | Path | Description |
| :--- | :--- | :--- |
| `GET` | `/` | Home page |
| `GET` | `/product` | Product listing with filters |
| `GET` | `/product/:id` | Product detail page |
| `GET/POST` | `/signup` | User registration |
| `GET/POST` | `/login` | User login |
| `GET` | `/auth/google` | Google OAuth initiation |
| `GET/POST` | `/verify-otp` | OTP verification |
| `GET/POST` | `/forgot-password` | Password reset flow |

### Protected User Routes (~30 routes)
Cart, checkout, orders, profile, address, wishlist, wallet — all behind JWT `User` middleware.

### Admin Routes (~50 routes)
Dashboard, product CRUD, category management, order processing, coupon management, user management, wallet oversight — all behind JWT `Admin` middleware.

---

## Logging

Structured logging via **Uber-Zap** (`pkg/logger`):

- **Development**: Human-readable console output with stack traces
- **Production**: JSON-formatted logs suitable for ELK Stack, CloudWatch, or Datadog

---

## Deployment Architecture

```
                    ┌─────────────┐
                    │   Nginx     │
                    │  (Reverse   │
                    │   Proxy)    │
                    └──────┬──────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
        ┌─────┴─────┐ ┌───┴─────┐ ┌───┴─────┐
        │  Pod (Go)  │ │ Pod (Go)│ │ Pod (Go)│
        │  :8080     │ │ :8080   │ │ :8080   │
        └─────┬──────┘ └───┬─────┘ └───┬─────┘
              │            │            │
              └────────────┼────────────┘
                           │
                    ┌──────┴──────┐
                    │ PostgreSQL  │
                    │  (RDS)      │
                    └─────────────┘
```

All pods run the **same monolithic binary**. Scaling is horizontal (add more replicas of the same application). There is no service-to-service communication — everything is in-process.

---

## License

This project is open source and available under the [MIT License](LICENSE).

---

*Built by [Mohammad Niyas](https://github.com/Mohammad-Niyas)*