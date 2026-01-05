# FinanceTracker
Personal finance management system with budgeting, analytics, and shared accounts. Built with Go and PostgreSQL.

FinanceTracker is a personal finance management system designed to help individuals and small groups track income and expenses, manage budgets, and gain insights into their financial activity.

The project supports shared accounts (for families or teams), category-based budgeting, and basic financial analytics. It is built as a backend-first application with a focus on clean architecture, security, and scalability.

This repository represents an MVP implementation intended as a pet project and a foundation for future growth into a fully distributed, microservices-based system.

## Key Features
- User authentication with JWT (access & refresh tokens)
- Personal and shared workspaces (family or team accounts)
- Income and expense tracking with categories
- Monthly budgets with overspending detection
- Basic financial analytics and statistics
- Role-based access control (owner, member, viewer)

## Tech Stack
- **Language:** Go (Golang)
- **Web Framework:** Gin
- **Database:** PostgreSQL
- **Caching:** Redis (optional, for analytics)
- **Authentication:** JWT + bcrypt
- **Infrastructure:** Docker, docker-compose
- **Testing:** Unit and integration tests

## Project Status
🚧 **MVP in active development**

The initial goal is to deliver a stable and functional MVP covering core finance tracking features. Advanced features such as notifications, bank integrations, and full microservices separation are planned for future iterations.

## License
This project is licensed under the MIT License.
