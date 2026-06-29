import { trace } from '@opentelemetry/api';

const tracer = trace.getTracer('node-checkout');
const span = tracer.startSpan('reconcile-orders');
span.setAttribute('user.id', '42');
span.end();

export const resource = {
  'service.name': 'node-checkout'
};
