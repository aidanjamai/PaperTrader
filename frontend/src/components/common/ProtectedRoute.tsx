import React from 'react';
import { Navigate } from 'react-router-dom';
import { User } from '../../types';

interface ProtectedRouteProps {
  isAuthenticated: boolean;
  user: User | null;
  children: React.ReactElement;
  redirectTo?: string;
}

/**
 * ProtectedRoute component
 * 
 * Wraps routes that require authentication
 * Redirects to login if user is not authenticated
 */
const ProtectedRoute: React.FC<ProtectedRouteProps> = ({
  isAuthenticated,
  user,
  children,
  redirectTo = '/login'
}) => {
  if (!isAuthenticated || !user) {
    return <Navigate to={redirectTo} replace />;
  }

  return children;
};

export default ProtectedRoute;

