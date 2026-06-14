FROM node:22-slim
# claude CLI for subscription-OAuth reasoning (raw API 429s on sub tokens)
RUN npm install -g @anthropic-ai/claude-code
RUN apt-get update && apt-get install -y --no-install-recommends python3 python3-pip python3-venv \
 && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY brainsvc/requirements.txt .
# CPU-only torch: the default wheel pulls multi-GB NVIDIA CUDA/cuDNN libs that are
# useless on this CPU box (and overflow its disk). Install the cpu wheel first;
# requirements.txt's torch==2.5.* is then already satisfied.
RUN python3 -m venv /venv \
 && /venv/bin/pip install --no-cache-dir torch==2.5.* --index-url https://download.pytorch.org/whl/cpu \
 && /venv/bin/pip install --no-cache-dir -r requirements.txt
COPY brainsvc/ ./brainsvc/
ENV HF_HOME=/data/hf
USER node
EXPOSE 8000
CMD ["/venv/bin/uvicorn", "brainsvc.app:app", "--host", "0.0.0.0", "--port", "8000"]
