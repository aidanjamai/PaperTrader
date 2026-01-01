import React, { useState, FormEvent, ChangeEvent, useMemo } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useGoogleLogin } from '@react-oauth/google';
import { apiRequest } from '../../services/api';
import { User, RegisterRequest, AuthResponse } from '../../types';
import { validatePassword, validateEmail } from '../../utils/validation';

interface RegisterProps {
  onLogin: (user: User) => void;
}

const Register: React.FC<RegisterProps> = ({ onLogin }) => {
  const [formData, setFormData] = useState<RegisterRequest>({
    email: '',
    password: ''
  });
  const [confirmPassword, setConfirmPassword] = useState<string>('');
  const [error, setError] = useState<string>('');
  const [loading, setLoading] = useState<boolean>(false);
  const [googleLoading, setGoogleLoading] = useState<boolean>(false);
  const navigate = useNavigate();

  const handleGoogleLogin = useGoogleLogin({
    onSuccess: async (tokenResponse: { access_token: string }) => {
      setGoogleLoading(true);
      setError('');
      try {
        const response = await apiRequest<AuthResponse>('/account/auth/google', {
          method: 'POST',
          body: JSON.stringify({ token: tokenResponse.access_token })
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
    },
    onError: () => {
      setError('Google login failed');
      setGoogleLoading(false);
    }
  });

  // Real-time password validation
  const passwordValidation = useMemo(() => {
    if (!formData.password) {
      return { isValid: false, errors: [] };
    }
    return validatePassword(formData.password);
  }, [formData.password]);

  // Check if passwords match
  const passwordsMatch = useMemo(() => {
    if (!formData.password || !confirmPassword) {
      return true; // Don't show error until both are filled
    }
    return formData.password === confirmPassword;
  }, [formData.password, confirmPassword]);

  // Check if email is valid
  const emailValidation = useMemo(() => {
    if (!formData.email) {
      return { isValid: true }; // Don't validate empty email
    }
    return validateEmail(formData.email);
  }, [formData.email]);

  // Check if form is valid (all requirements met)
  const isFormValid = useMemo(() => {
    return (
      emailValidation.isValid &&
      passwordValidation.isValid &&
      passwordsMatch &&
      formData.password.length > 0 &&
      confirmPassword.length > 0
    );
  }, [emailValidation.isValid, passwordValidation.isValid, passwordsMatch, formData.password, confirmPassword]);

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

    // Final validation check
    if (!isFormValid) {
      if (!passwordsMatch) {
        setError('Passwords do not match');
      } else if (!passwordValidation.isValid) {
        setError(passwordValidation.errors.join(', '));
      } else if (!emailValidation.isValid) {
        setError(emailValidation.error || 'Invalid email');
      }
      return;
    }

    setLoading(true);

    try {
      const response = await apiRequest<AuthResponse>('/account/register', {
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
        setError(data.message || 'Registration failed');
      }
    } catch (error) {
      setError('Network error. Please try again.');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ marginTop: '60px' }}>
      <div className="card">
        <h2>Create Account</h2>
        
        {error && (
          <div className="alert alert-error">
            {error}
          </div>
        )}

        <button
          type="button"
          onClick={() => handleGoogleLogin()}
          className="btn btn-secondary"
          style={{ 
            width: '100%', 
            marginBottom: '16px',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            gap: '8px'
          }}
          disabled={loading || googleLoading}
        >
          <svg width="18" height="18" viewBox="0 0 18 18" style={{ flexShrink: 0 }}>
            <path fill="#4285F4" d="M17.64 9.2c0-.637-.057-1.251-.164-1.84H9v3.481h4.844c-.209 1.125-.843 2.078-1.796 2.717v2.258h2.908c1.702-1.567 2.684-3.874 2.684-6.615z"/>
            <path fill="#34A853" d="M9 18c2.43 0 4.467-.806 5.956-2.184l-2.908-2.258c-.806.54-1.837.86-3.048.86-2.344 0-4.328-1.584-5.036-3.711H.957v2.332C2.438 15.983 5.482 18 9 18z"/>
            <path fill="#FBBC05" d="M3.964 10.712c-.18-.54-.282-1.117-.282-1.712 0-.595.102-1.172.282-1.712V4.956H.957C.348 6.174 0 7.55 0 9c0 1.45.348 2.826.957 4.044l3.007-2.332z"/>
            <path fill="#EA4335" d="M9 3.58c1.321 0 2.508.454 3.44 1.345l2.582-2.58C13.463.891 11.426 0 9 0 5.482 0 2.438 2.017.957 4.956L3.964 7.288C4.672 5.163 6.656 3.58 9 3.58z"/>
          </svg>
          {googleLoading ? 'Signing in...' : 'Register with Google'}
        </button>

        <div style={{ 
          display: 'flex', 
          alignItems: 'center', 
          marginBottom: '16px',
          textAlign: 'center'
        }}>
          <div style={{ flex: 1, height: '1px', backgroundColor: '#ddd' }}></div>
          <span style={{ padding: '0 16px', color: '#666' }}>or</span>
          <div style={{ flex: 1, height: '1px', backgroundColor: '#ddd' }}></div>
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
              disabled={loading || googleLoading}
            />
          </div>

          <div className="form-group">
            <label htmlFor="password">Password</label>
            <input
              type="password"
              id="password"
              name="password"
              className={`form-control ${formData.password && !passwordValidation.isValid ? 'is-invalid' : formData.password && passwordValidation.isValid ? 'is-valid' : ''}`}
              value={formData.password}
              onChange={handleChange}
              required
              disabled={loading || googleLoading}
            />
            {formData.password && (
              <div className="mt-2" style={{ fontSize: '0.875rem' }}>
                <div style={{ marginBottom: '8px', fontWeight: '500' }}>Password requirements:</div>
                <ul style={{ margin: 0, paddingLeft: '20px', listStyle: 'none' }}>
                  <li style={{ color: formData.password.length >= 8 ? '#28a745' : '#dc3545' }}>
                    {formData.password.length >= 8 ? '✓' : '✗'} At least 8 characters
                  </li>
                  <li style={{ color: /[A-Z]/.test(formData.password) ? '#28a745' : '#dc3545' }}>
                    {/[A-Z]/.test(formData.password) ? '✓' : '✗'} One uppercase letter
                  </li>
                  <li style={{ color: /[a-z]/.test(formData.password) ? '#28a745' : '#dc3545' }}>
                    {/[a-z]/.test(formData.password) ? '✓' : '✗'} One lowercase letter
                  </li>
                  <li style={{ color: /[0-9]/.test(formData.password) ? '#28a745' : '#dc3545' }}>
                    {/[0-9]/.test(formData.password) ? '✓' : '✗'} One number
                  </li>
                  <li style={{ color: /[!@#$%^&*()\-_+=[\]{}|\\:;"'<>,.?/~`]/.test(formData.password) ? '#28a745' : '#dc3545' }}>
                    {/[!@#$%^&*()\-_+=[\]{}|\\:;"'<>,.?/~`]/.test(formData.password) ? '✓' : '✗'} One special character
                  </li>
                </ul>
              </div>
            )}
          </div>

          <div className="form-group">
            <label htmlFor="confirmPassword">Confirm Password</label>
            <input
              type="password"
              id="confirmPassword"
              name="confirmPassword"
              className={`form-control ${confirmPassword && !passwordsMatch ? 'is-invalid' : ''}`}
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              required
              disabled={loading || googleLoading}
            />
            {confirmPassword && !passwordsMatch && (
              <div style={{ display: 'block', color: '#dc3545', fontSize: '0.875rem', marginTop: '0.25rem' }}>
                Passwords do not match
              </div>
            )}
          </div>

          <button
            type="submit"
            className={isFormValid ? 'btn btn-primary' : 'btn btn-secondary'}
            style={{ 
              width: '100%',
              ...(isFormValid ? {} : { 
                backgroundColor: '#6c757d',
                borderColor: '#6c757d',
                color: '#fff',
                cursor: 'not-allowed',
                opacity: 0.65
              })
            }}
            disabled={loading || !isFormValid}
            onMouseEnter={(e) => {
              if (!isFormValid) {
                e.currentTarget.style.backgroundColor = '#6c757d';
                e.currentTarget.style.borderColor = '#6c757d';
              }
            }}
            onMouseLeave={(e) => {
              if (!isFormValid) {
                e.currentTarget.style.backgroundColor = '#6c757d';
                e.currentTarget.style.borderColor = '#6c757d';
              }
            }}
          >
            {loading ? 'Creating Account...' : 'Create Account'}
          </button>
        </form>

        <div style={{ textAlign: 'center', marginTop: '24px' }}>
          <p style={{ color: '#666', marginBottom: '16px' }}>
            Already have an account?
          </p>
          <Link to="/login" className="btn btn-secondary">
            Sign In
          </Link>
        </div>
      </div>
    </div>
  );
};

export default Register;

