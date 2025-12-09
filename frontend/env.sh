#!/bin/sh

# Recreate config file
rm -rf /usr/share/nginx/html/env-config.js
touch /usr/share/nginx/html/env-config.js

# Add assignment 
echo "window.env = {" >> /usr/share/nginx/html/env-config.js

# Read each line in .env file
# Each line represents key=value pairs
printenv | grep REACT_APP_ | awk -F = '{ print "  \"" $1 "\": \"" $2 "\"," }' >> /usr/share/nginx/html/env-config.js

echo "}" >> /usr/share/nginx/html/env-config.js

# Start Nginx
exec "$@"

