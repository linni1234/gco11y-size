import express from 'express';
import axios from 'axios';

const app = express();
const router = express.Router();

router.get('/orders/:id', (req, res) => res.json({ id: req.params.id }));
router.post('/orders', (req, res) => res.status(201).end());
app.use('/api', router);
app.route('/legacy/:id').delete((req, res) => res.end());

await axios.get('https://inventory-service.internal/api/items');

export default app;
