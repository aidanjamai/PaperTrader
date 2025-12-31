#!/usr/bin/env node

/**
 * Script to sync REACT_APP_* environment variables from root .env to frontend/.env
 * This allows us to maintain a single .env file in the root directory
 * 
 * Note: This script is only needed for local development.
 * In Docker, environment variables are passed directly via docker-compose.
 */

const fs = require('fs');
const path = require('path');

// Handle both local and Docker paths
// In Docker, __dirname might be /app, so we need to check
let rootEnvPath, frontendEnvPath;

if (fs.existsSync(path.join(__dirname, '..', '.env'))) {
  // Local development - scripts is in root/scripts
  rootEnvPath = path.join(__dirname, '..', '.env');
  frontendEnvPath = path.join(__dirname, '..', 'frontend', '.env');
} else if (fs.existsSync(path.join(__dirname, '..', '..', '.env'))) {
  // Docker - might be in /app/scripts or similar
  rootEnvPath = path.join(__dirname, '..', '..', '.env');
  frontendEnvPath = path.join(__dirname, '..', '.env');
} else {
  // Try to find .env in parent directories
  let currentDir = __dirname;
  for (let i = 0; i < 5; i++) {
    const potentialEnv = path.join(currentDir, '.env');
    if (fs.existsSync(potentialEnv)) {
      rootEnvPath = potentialEnv;
      // Look for frontend directory
      const potentialFrontend = path.join(currentDir, 'frontend', '.env');
      if (fs.existsSync(path.dirname(potentialFrontend))) {
        frontendEnvPath = potentialFrontend;
        break;
      }
    }
    currentDir = path.join(currentDir, '..');
  }
}

// Read root .env file
if (!rootEnvPath || !fs.existsSync(rootEnvPath)) {
  // In Docker, we don't need to sync - env vars are passed directly
  // Just exit silently
  process.exit(0);
}

if (!frontendEnvPath) {
  // Can't determine frontend path, exit silently (probably in Docker)
  process.exit(0);
}

const rootEnvContent = fs.readFileSync(rootEnvPath, 'utf8');
const lines = rootEnvContent.split('\n');

// Extract REACT_APP_* variables and comments
const reactEnvVars = [];
let inReactSection = false;

lines.forEach((line, index) => {
  const trimmed = line.trim();
  
  // Include comments related to frontend/React
  if (trimmed.startsWith('#') && (
    trimmed.toLowerCase().includes('frontend') ||
    trimmed.toLowerCase().includes('react') ||
    trimmed.toLowerCase().includes('google') ||
    trimmed.toLowerCase().includes('api_url')
  )) {
    reactEnvVars.push(line);
    inReactSection = true;
  }
  // Include REACT_APP_* variables
  else if (trimmed.startsWith('REACT_APP_')) {
    reactEnvVars.push(line);
    inReactSection = true;
  }
  // Reset section flag on blank lines (but keep them if we're in a React section)
  else if (trimmed === '' && inReactSection && index < lines.length - 1) {
    // Check if next line is also React-related
    const nextLine = lines[index + 1];
    if (!nextLine.trim().startsWith('REACT_APP_') && 
        !nextLine.trim().startsWith('#') &&
        nextLine.trim() !== '') {
      inReactSection = false;
    }
    reactEnvVars.push(line);
  }
  else if (inReactSection && trimmed === '') {
    reactEnvVars.push(line);
  }
  else {
    inReactSection = false;
  }
});

// Create frontend .env content
const frontendEnvContent = `# Frontend Environment Variables
# This file is auto-generated from root .env
# Do not edit manually - edit root .env and run: npm run sync-env
# Or run: node scripts/sync-env.js

${reactEnvVars.filter(line => line.trim() !== '').join('\n')}
`;

// Write to frontend/.env
fs.writeFileSync(frontendEnvPath, frontendEnvContent, 'utf8');
console.log('✅ Synced REACT_APP_* variables from root .env to frontend/.env');
console.log(`   Found ${reactEnvVars.filter(line => line.startsWith('REACT_APP_')).length} REACT_APP_* variable(s)`);

