# VogueLuxe E-Commerce Platform: Technical Documentation

A robust, enterprise-grade e-commerce engine built using **Golang** and the **MVC (Model-View-Controller)** architecture. This platform is designed for high-performance retail operations, featuring scalable service integrations and a secure, cloud-native deployment strategy.

---

## 🛠️ Technology Stack

### Core Backend
- **Language**: Go (v1.24.0) — Chosen for its efficiency in concurrent processing and low memory footprint.
- **Web Framework**: [Gin-Gonic v1.11](https://gin-gonic.com/) — A high-performance HTTP web framework with a minimalist API.
- **ORM**: [GORM v1.25+](https://gorm.io/) — Used for streamlined database interactions and automated schema migrations.
- **Database**: [PostgreSQL](https://www.postgresql.org/) — Relational database management system for transactional data integrity.

### Integrated Services
- **Payments**: [Razorpay API](https://razorpay.com/) — Handles secure checkouts, payment verification, and transaction logging.
- **Authentication**: 
  - **JWT (JSON Web Tokens)** — For stateless session management between the client and server.
  - **Google OAuth 2.0** — Social login integration for enhanced user accessibility.
- **Communications**: [Brevo (formerly Sendinblue)](https://www.brevo.com/) — SMTP/API service for transactional emails and OTP (One-Time Password) generation.
- **CDN/Storage**: [Cloudinary](https://cloudinary.com/) — Automated image optimization and cloud storage for product assets.
- **Logging**: [Uber-Zap](https://github.com/uber-go/zap) — Blazing fast, structured, leveled logging for production debugging.

### Frontend
- **Templating**: Gin's built-in `html/template` engine for server-side rendering (SSR).
- **Styling**: [Tailwind CSS](https://tailwindcss.com/) — Utility-first CSS framework for responsive and modern UI design.

---

## 🏗️ Architectural Overview

The project follows a strict **MVC (Model-View-Controller)** pattern to ensure a clean separation of concerns and maintainability.

### Data Flow Pattern
1. **Client Request**: Originates from the user's browser or an API client.
2. **Middleware**: JWT verification, role-based access control (RBAC), and request logging occur before reaching the controller.
3. **Controller**: Validates entry parameters and invokes the necessary business logic.
4. **Model/Service**: Interactions with PostgreSQL via GORM or external APIs (Razorpay/Cloudinary).
5. **View/Response**: The server renders an HTML template (using Gin's `LoadHTMLGlob`) or returns a structured JSON response.

### Core Modules
| Module | Description |
| :--- | :--- |
| **User & Auth** | Signup/Login with OTP verification, Google OAuth integration, and JWT session handling. |
| **Catalog System** | Multi-level category management with dynamic product filtering and search. |
| **Order Engine** | Transactional flow from cart to fulfillment, including PDF invoice generation. |
| **Inventory** | Real-time stock tracking with automated updates upon order completion. |
| **Promotions** | Coupon management system with validity checks and discount calculations. |
| **Wallets** | Integrated credit system for users to store and spend balance within the app. |

---

## 📂 Project Structure

```text
Ecommerce-Project/
├── config/             # Configuration logic (DB, Env, Google Auth)
├── controllers/        # Business logic for Admin, Users, Products, etc.
├── k8s/                # Kubernetes Deployment & Service manifests
├── middleware/         # Auth (JWT), Logging, and UI Access Checkers
├── models/             # GORM Entity definitions and Database schema
├── pkg/                # Reusable packages (Logger setup)
├── routers/            # Route definitions for User and Admin panels
├── utils/              # Third-party integrations (Cloudinary, Razorpay, OTP)
├── views/              # SSR HTML Templates (Admin/User subdirectories)
├── main.go             # Application entry point & Server initialization
├── go.mod/go.sum       # Dependency management
├── Dockerfile          # Containerization manifest
└── Makefile            # Build and Automation commands
```

---

## 🚀 Getting Started

### 📋 Prerequisites
- **Go 1.24+**: For compiling and running the backend.
- **PostgreSQL**: A running instance with a dedicated database.
- **Docker/K8s** (Optional): For containerized local development or production clusters.

### ⚙️ Environment Configuration
Create a `.env` file in the root directory with the following keys:
```env
# Server
PORT=8080
SECRETKEY="your_secure_jwt_key"

# Database
DB="host=... user=... password=... dbname=... port=..."

# External APIs
CLOUDINARY_CLOUD_NAME="..."
CLOUDINARY_API_KEY="..."
CLOUDINARY_API_SECRET="..."
BREVO_API_KEY="..."
GOOGLE_CLIENT_ID="..."
GOOGLE_CLIENT_SECRET="..."
RAZORPAY_KEY_ID="..."
RAZORPAY_KEY_SECRET="..."
```

### 🛠️ Installation & Run
1. **Fetch Dependencies**:
   ```bash
   go mod tidy
   ```
2. **Launch Application**:
   ```bash
   go run main.go
   ```
3. **Production Build**:
   ```bash
   make build
   ```

---

## 🌐 Deployment Strategy

### Containerization
The project is fully **Dockerized**, allowing for consistent environment behavior from development to production.
- Use `docker build -t vogueluxe .` to create the image.

### Orchestration
**Kubernetes (K8s)** manifests are provided for automated scaling and zero-downtime deployments:
- `deployment.yaml`: Manages the application pods, replicas, and container images.
- `service.yaml`: Exposes the application to internal/external traffic via LoadBalancers.

### Infrastructure
Designed for deployment on **AWS (EC2/RDS)**:
- **Nginx** is recommended as a reverse proxy for TLS termination and performance optimization.
- **GORM AutoMigrate** ensures the RDS schema is always in sync with the codebase.

---

## 📈 Logging & Debugging
The platform uses the **Uber-Zap Logger** located in `pkg/logger`.
- **In Development**: Pretty-printed console logs with detailed stack traces.
- **In Production**: Structured JSON logging for integration with ELK stack or AWS CloudWatch.

---
*Maintained by [Mohammad Niyas](https://github.com/Mohammad-Niyas)*