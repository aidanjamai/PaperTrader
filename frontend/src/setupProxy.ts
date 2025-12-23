import { createProxyMiddleware } from 'http-proxy-middleware';
import type { Application } from 'express';

/**
 * Development proxy configuration
 * 
 * Proxies API requests to the backend server
 */
module.exports = function(app: Application) {
  app.use(
    '/api',
    createProxyMiddleware({
      target: process.env.BACKEND_URL || 'http://backend:8080',
      changeOrigin: true,
    })
  );
};

