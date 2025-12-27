const { createProxyMiddleware } = require('http-proxy-middleware');

/**
 * Development proxy configuration
 * 
 * Proxies API requests to the backend server
 */
module.exports = function(app) {
  app.use(
    '/api',
    createProxyMiddleware({
      target: process.env.BACKEND_URL || 'http://backend:8080',
      changeOrigin: true,
      logLevel: 'debug',
      secure: false,
    })
  );
};


