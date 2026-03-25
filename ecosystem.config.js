module.exports = {
  apps: [
    {
      name: 'kyron-frontend',
      script: 'npm',
      args: 'start',
      cwd: '/home/ubuntu/app/frontend',
      instances: 1,
      autorestart: true,
      watch: false,
      env: {
        NODE_ENV: 'production',
        PORT: 3000,
      },
    },
  ],
};
