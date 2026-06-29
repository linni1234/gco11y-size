import fastify from 'fastify';

const app = fastify();

app.get('/fast/health', async () => ({ ok: true }));
app.route({
  method: 'POST',
  url: '/fast/orders/:id/cancel',
  handler: async () => ({ ok: true })
});

export default app;
