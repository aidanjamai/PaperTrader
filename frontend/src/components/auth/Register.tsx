import React, { useState, FormEvent, ChangeEvent, useMemo } from 'react';
import { Link, useNavigate } from 'react-router-dom';
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
  const navigate = useNavigate();

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
              className={`form-control ${formData.password && !passwordValidation.isValid ? 'is-invalid' : formData.password && passwordValidation.isValid ? 'is-valid' : ''}`}
              value={formData.password}
              onChange={handleChange}
              required
              disabled={loading}
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
              disabled={loading}
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

