# PaperTrader - Authentication System

A simple Go backend with React frontend for user authentication (register, login, logout).

## Features

- **Backend**: Go with Gorilla Mux and SQLite
- **Frontend**: React with modern UI design
- **Authentication**: Secure password hashing with bcrypt
- **Sessions**: Cookie-based session management
- **Database**: SQLite for data persistence
- **User Model**: UUID-based identification with balance tracking

## Project Structure

```
PaperTrader/
├── backend/
│   ├── main.go                 # Main server file
│   ├── go.mod                  # Go dependencies
│   └── internal/
│       ├── api/
│       │   └── account.go      # Authentication handlers
│       └── data/
│           └── user.go         # User model and database operations
└── frontend/
    ├── package.json            # React dependencies
    ├── public/
    │   └── index.html          # Main HTML file
    └── src/
        ├── App.js              # Main React component
        ├── index.js            # React entry point
        ├── App.css             # App-specific styles
        ├── index.css           # Global styles
        └── components/
            ├── Navbar.js       # Navigation component
            ├── Home.js         # Home page
            ├── Login.js        # Login form
            ├── Register.js     # Registration form
            └── Dashboard.js    # User dashboard
```

## User Model

The system uses a simplified user model with the following fields:

- **ID**: Randomly generated UUID (primary key)
- **Email**: Unique email address for authentication
- **Password**: Securely hashed password
- **CreatedAt**: Account creation timestamp
- **Balance**: Starting balance of $10,000.00 for paper trading

## Setup Instructions

### Backend Setup

1. **Navigate to backend directory:**
   ```bash
   cd backend
   ```

2. **Install Go dependencies:**
   ```bash
   go mod tidy
   ```

3. **Run the server:**
   ```bash
   go run main.go
   ```http://localhost:8080

   The server will start on ``

### Frontend Setup

1. **Navigate to frontend directory:**
   ```bash
   cd frontend
   ```

2. **Install Node.js dependencies:**
   ```bash
   npm install
   ```

3. **Start the development server:**
   ```bash
   npm start
   ```

   The React app will open in your browser at `http://localhost:3000`

Local development : docker compose -f docker-compose.dev.yml up --build

VM: sudo docker compose up -d


## API Endpoints

### Authentication

- `POST /api/auth/register` - User registration
- `POST /api/auth/login` - User login
- `POST /api/auth/logout` - User logout
- `GET /api/auth/profile` - Get user profile
- `GET /api/auth/check` - Check authentication status

### Request/Response Examples

#### Register
```json
POST /api/auth/register
{
  "email": "john@example.com",
  "password": "password123"
}
```

#### Login
```json
POST /api/auth/login
{
  "email": "john@example.com",
  "password": "password123"
}
```

## Features

### Backend
- Secure password hashing with bcrypt
- SQLite database with automatic table creation
- Session-based authentication
- CORS support for frontend integration
- Input validation and error handling
- UUID-based user identification
- Balance tracking for paper trading

### Frontend
- Modern, responsive UI design
- Form validation
- Protected routes
- Session management
- Error handling and user feedback
- Balance display in dashboard

## Security Features

- Passwords are hashed using bcrypt
- Session cookies for authentication
- Input validation and sanitization
- CORS configuration for security
- UUID-based user identification

## Development

### Adding New Features

1. **Backend**: Add new handlers in `internal/api/` and routes in `main.go`
2. **Frontend**: Create new components in `src/components/` and add routes in `App.js`

### Database Changes

- Modify the `Init()` function in `internal/data/user.go`
- The database will automatically create/update tables on startup

## Troubleshooting

### Common Issues

1. **Port already in use**: Change the port in `main.go` or kill the process using the port
2. **Database errors**: Delete `papertrader.db` file and restart the server
3. **CORS issues**: Ensure the frontend is running on `http://localhost:3000`

### Logs

- Backend logs are printed to the console
- Check the terminal where you ran `go run main.go`

## Next Steps

This is a basic authentication system. You can extend it by adding:

- Password reset functionality
- Email verification
- User roles and permissions
- Trading functionality
- Portfolio management
- Real-time market data
- Balance updates after trades




1. Robust Distributed Transaction Safety
Problem: Previously, buying a stock involved separate, unconnected operations to SQLite (deducting money) and MongoDB (adding stock). If the server crashed in between, a user could lose money without getting the stock.
Solution: I implemented a Two-Phase-Like Commit flow in the new InvestmentService:
SQLite Transaction: Deduct balance and create a PENDING trade record. Commit this transaction (money is "reserved").
MongoDB Operation: Add the stock to the user's portfolio.
Finalization: Update the trade record to COMPLETED in SQLite.
Safety: If MongoDB fails, the trade remains PENDING in the database, allowing for future reconciliation (and the user hasn't "lost" the record of the transaction).
2. Professional Service Layer Architecture
Standardization: I moved business logic out of the HTTP handlers (api/) and into a dedicated internal/service layer.
AuthService: Handles registration, login, and token validation.
MarketService: Handles stock data fetching, caching, and validation.
InvestmentService: Handles the complex buy/sell logic and transaction management.
Benefits: This makes the code testable, reusable, and cleanly separated from the transport layer (HTTP).
3. Database Transaction Support
I refactored UserStore, TradeStore, and StockStore to accept a DBTX interface. This allows them to participate in the same database transaction managed by the Service layer, ensuring atomicity for critical operations like "deduct balance + create trade".
4. Frontend "Real-Data" Integration
Dashboard: The dashboard now fetches real portfolio data from the backend. It dynamically calculates your:
Portfolio Value: Based on your actual holdings.
Total Net Worth: Cash + Investments.
Holdings Table: Displays a list of stocks you own with live calculations.
5. Code Cleanup & Precision
Rounding: Implemented strict 2-decimal rounding for financial calculations to prevent floating-point errors (e.g., $100.00000001).
Cleanup: Removed redundant code in api/auth and centralized logic in the new services.
You can now restart your application to see these changes in action. The database schema for trades will automatically migrate to include the new status column.




## License

This project is open source and available under the MIT License.
