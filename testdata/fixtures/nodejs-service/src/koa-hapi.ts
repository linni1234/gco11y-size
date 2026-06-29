import Koa from 'koa';
import Router from '@koa/router';
import Hapi from '@hapi/hapi';

const koaRouter = new Router();
koaRouter.put('/koa/orders/:id', ctx => {});

const server = Hapi.server();
server.route({
  method: 'GET',
  path: '/hapi/orders/{id}',
  handler: () => ({})
});
