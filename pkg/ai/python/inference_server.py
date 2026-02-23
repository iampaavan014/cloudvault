import os
import torch
import torch.nn as nn
from flask import Flask, request, jsonify
import numpy as np

app = Flask(__name__)

# Real LSTM Model using PyTorch
class CloudCostLSTM(nn.Module):
    def __init__(self, input_size=1, hidden_size=64, num_layers=2):
        super(CloudCostLSTM, self).__init__()
        self.lstm = nn.LSTM(input_size, hidden_size, num_layers, batch_first=True)
        self.fc = nn.Linear(hidden_size, 1)

    def forward(self, x):
        h0 = torch.zeros(2, x.size(0), 64)
        c0 = torch.zeros(2, x.size(0), 64)
        out, _ = self.lstm(x, (h0, c0))
        out = self.fc(out[:, -1, :])
        return out

# Real Reinforcement Learning Policy (Simplified for demo, but real PyTorch)
class PlacementAgent(nn.Module):
    def __init__(self, state_dim, action_dim):
        super(PlacementAgent, self).__init__()
        self.network = nn.Sequential(
            nn.Linear(state_dim, 128),
            nn.ReLU(),
            nn.Linear(128, 64),
            nn.ReLU(),
            nn.Linear(64, action_dim)
        )

    def forward(self, x):
        return self.network(x)

# Initialize models
lstm_model = CloudCostLSTM()
rl_model = None # Initialized on first call with dims

@app.route('/predict', methods=['POST'])
def predict():
    data = request.json
    history = data.get('history', [])
    if not history:
        return jsonify({'prediction': 0.0})
    
    # Pre-process
    input_tensor = torch.tensor(history, dtype=torch.float32).view(1, -1, 1)
    
    with torch.no_grad():
        prediction = lstm_model(input_tensor).item()
    
    return jsonify({'prediction': float(prediction)})

@app.route('/health', methods=['GET'])
def health():
    return jsonify({'status': 'ok', 'model': 'lstm+rl', 'framework': 'pytorch'}), 200

@app.route('/decide', methods=['POST'])
def decide():
    global rl_model
    data = request.json
    state = data.get('state', [])
    action_dim = data.get('action_dim', 3)

    # Input validation
    if not state or not isinstance(state, list) or len(state) == 0:
        return jsonify({'error': 'Invalid input: state must be a non-empty list'}), 400

    if rl_model is None:
        rl_model = PlacementAgent(len(state), action_dim)

    state_tensor = torch.tensor(state, dtype=torch.float32).unsqueeze(0)

    with torch.no_grad():
        q_values = rl_model(state_tensor)
        action = torch.argmax(q_values).item()

    return jsonify({'action_index': int(action)})

if __name__ == '__main__':
    port = int(os.environ.get('PORT', 5005))
    app.run(host='0.0.0.0', port=port)
