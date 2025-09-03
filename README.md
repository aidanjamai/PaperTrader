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
   ```

   The server will start on `http://localhost:8080`

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

## License

This project is open source and available under the MIT License.
