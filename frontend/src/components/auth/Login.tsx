import React, { useState, FormEvent, ChangeEvent } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { GoogleLogin, CredentialResponse } from '@react-oauth/google';
import { apiRequest } from '../../services/api';
import { User, LoginRequest, AuthResponse } from '../../types';

interface LoginProps {
  onLogin: (user: User) => void;
}

const Login: React.FC<LoginProps> = ({ onLogin }) => {
  const [formData, setFormData] = useState<LoginRequest>({
    email: '',
    password: ''
  });
  const [error, setError] = useState<string>('');
  const [loading, setLoading] = useState<boolean>(false);
  const [googleLoading, setGoogleLoading] = useState<boolean>(false);
  const navigate = useNavigate();

  const handleGoogleSuccess = async (credentialResponse: CredentialResponse) => {
    const credential = credentialResponse.credential;
    if (!credential) {
      setError('Google login failed: no credential returned');
      return;
    }
    setGoogleLoading(true);
    setError('');
    try {
      const response = await apiRequest<AuthResponse>('/account/auth/google', {
        method: 'POST',
        body: JSON.stringify({ token: credential })
      });

      const data = await response.json() as AuthResponse;
      if (response.ok && data.success && data.user) {
        localStorage.setItem('user', JSON.stringify(data.user));
        onLogin(data.user);
        navigate('/dashboard');
      } else {
        setError(data.message || 'Google login failed');
      }
    } catch (error) {
      console.error('Google login error:', error);
      setError('Failed to login with Google');
    } finally {
      setGoogleLoading(false);
    }
  };

  const handleChange = (e: ChangeEvent<HTMLInputElement>) => {
    const { name, value } = e.target;
    setFormData(prev => ({
      ...prev,
      [name]: value
    }));
  };

  const handleSubmit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      const response = await apiRequest<AuthResponse>('/account/login', {
        method: 'POST',
        body: JSON.stringify(formData)
      });

      const data = await response.json() as AuthResponse;

      if (response.ok && data.success && data.user) {
        // Token is set as HttpOnly cookie by backend, no need to store in localStorage
        localStorage.setItem('user', JSON.stringify(data.user));
        onLogin(data.user);
        navigate('/dashboard');
      } else {
        setError(data.message || 'Login failed');
      }
    } catch (error) {
      console.error('Login error:', error);
      const errorMessage = error instanceof Error ? error.message : 'Network error. Please try again.';
      setError(errorMessage);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="container-narrow" style={{ paddingTop: 40 }}>
      <div className="card">
        <h2>Log in</h2>
        
        {error && (
          <div className="alert alert-error">
            {error}
          </div>
        )}

        <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'center' }}>
          <GoogleLogin
            onSuccess={handleGoogleSuccess}
            onError={() => setError('Google login failed')}
            useOneTap={false}
          />
        </div>
        {googleLoading && (
          <div className="muted" style={{ textAlign: 'center', marginBottom: 16, fontSize: 13 }}>
            Signing in…
          </div>
        )}

        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            marginBottom: 16,
          }}
        >
          <div style={{ flex: 1, height: 1, background: 'var(--hairline)' }} />
          <span
            className="eyebrow"
            style={{ padding: '0 12px' }}
          >
            or
          </span>
          <div style={{ flex: 1, height: 1, background: 'var(--hairline)' }} />
        </div>

        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label htmlFor="email">Email</label>
            <input
              type="email"
              id="email"
              name="email"
              className="form-control"
              value={formData.email}
              onChange={handleChange}
              required
              disabled={loading}
            />
          </div>

          <div className="form-group">
            <label htmlFor="password">Password</label>
            <input
              type="password"
              id="password"
              name="password"
              className="form-control"
              value={formData.password}
              onChange={handleChange}
              required
              disabled={loading}
            />
          </div>

          <button
            type="submit"
            className="btn btn-primary"
            style={{ width: '100%' }}
            disabled={loading || googleLoading}
          >
            {loading ? 'Logging in…' : 'Log in'}
          </button>
        </form>

        <div style={{ textAlign: 'center', marginTop: 24 }}>
          <p className="muted" style={{ marginBottom: 12, fontSize: 13 }}>
            Don't have an account?
          </p>
          <Link to="/register" className="btn btn-secondary">
            Create account
          </Link>
        </div>
      </div>
    </div>
  );
};

export default Login;

