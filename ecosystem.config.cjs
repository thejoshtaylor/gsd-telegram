module.exports = {
  apps: [
    {
      name: "telegram-claude",
      script: "src/index.ts",
      interpreter: "node",
      interpreter_args: "--import tsx/esm",
      cwd: "D:/Projects/_ControlCenter/services/telegram-claude",
      restart_delay: 5000,
      max_restarts: 50,
      min_uptime: 10000,
      exp_backoff_restart_delay: 1000,
      watch: false,
      env: {
        NODE_ENV: "production",
      },
    },
  ],
};
