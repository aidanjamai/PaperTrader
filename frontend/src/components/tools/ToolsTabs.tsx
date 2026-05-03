import React from 'react';
import { NavLink } from 'react-router-dom';

const ToolsTabs: React.FC = () => (
  <nav className="tabs" aria-label="Tools" style={{ marginBottom: 16 }}>
    <NavLink
      to="/calculator"
      className={({ isActive }) => `tab${isActive ? ' active' : ''}`}
    >
      Portfolio calculator
    </NavLink>
    <NavLink
      to="/compound-interest"
      className={({ isActive }) => `tab${isActive ? ' active' : ''}`}
    >
      Compound interest
    </NavLink>
  </nav>
);

export default ToolsTabs;
