import React, { useState, FormEvent, ChangeEvent, useMemo } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { GoogleLogin, CredentialResponse } from '@react-oauth/google';
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
    <div className="container-narrow" style={{ paddingTop: 40 }}>
      <div className="card">
        <h2>Create account</h2>
        
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
            text="signup_with"
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
          <span className="eyebrow" style={{ padding: '0 12px' }}>
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
              <div style={{ fontSize: 13, marginTop: 10 }}>
                <div className="eyebrow" style={{ marginBottom: 6 }}>
                  Password requirements
                </div>
                <ul style={{ margin: 0, paddingLeft: 0, listStyle: 'none' }}>
                  {[
                    { ok: formData.password.length >= 8, label: 'At least 8 characters' },
                    { ok: /[A-Z]/.test(formData.password), label: 'One uppercase letter' },
                    { ok: /[a-z]/.test(formData.password), label: 'One lowercase letter' },
                    { ok: /[0-9]/.test(formData.password), label: 'One number' },
                    {
                      ok: /[!@#$%^&*()\-_+=[\]{}|\\:;"'<>,.?/~`]/.test(formData.password),
                      label: 'One special character',
                    },
                  ].map((req, i) => (
                    <li
                      key={i}
                      className="mono"
                      style={{
                        color: req.ok ? 'var(--gain)' : 'var(--ink-muted)',
                        fontSize: 12,
                        lineHeight: 1.7,
                      }}
                    >
                      {req.ok ? '✓' : '·'} {req.label}
                    </li>
                  ))}
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
              <div className="loss-text" style={{ fontSize: 12, marginTop: 6 }}>
                Passwords do not match.
              </div>
            )}
          </div>

          <button
            type="submit"
            className="btn btn-primary btn-block"
            disabled={loading || !isFormValid}
          >
            {loading ? 'Creating account…' : 'Create account'}
          </button>
        </form>

        <div style={{ textAlign: 'center', marginTop: 24 }}>
          <p className="muted" style={{ marginBottom: 12, fontSize: 13 }}>
            Already have an account?
          </p>
          <Link to="/login" className="btn btn-secondary">
            Sign in
          </Link>
        </div>
      </div>
    </div>
  );
};

export default Register;

