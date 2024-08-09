#!/bin/sh
# entrypoint.sh
# Add the cron job
echo "0 3 * * * /backup.sh" >> /etc/crontabs/root
# Start cron
crond -f
