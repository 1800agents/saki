import { createApp } from './app';
import { config } from './config/env';

const app = createApp();

app.listen(config.port, config.host, () => {
  console.log(`saki control-plane listening on http://${config.host}:${config.port}`);
});
