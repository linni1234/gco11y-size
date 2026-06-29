export default function handler(req, res) {
  if (req.method === 'POST') {
    res.status(201).json({});
    return;
  }
  if (req.method === 'GET') {
    res.status(200).json({});
  }
}
