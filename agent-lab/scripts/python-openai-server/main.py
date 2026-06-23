import argparse
import json
import os
import sys
import threading
import time
import uuid
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

MODEL = None
TOKENIZER = None
MODEL_ID = None
DEVICE = None
MODEL_LOCK = threading.Lock()


def load_model(model_id, device):
    global MODEL, TOKENIZER, DEVICE, MODEL_ID
    with MODEL_LOCK:
        if MODEL is not None and TOKENIZER is not None and MODEL_ID == model_id:
            return TOKENIZER, MODEL, DEVICE
        try:
            import torch
            from transformers import AutoModelForCausalLM, AutoTokenizer
        except ModuleNotFoundError as exc:
            name = exc.name or str(exc)
            raise RuntimeError(
                f"missing python dependency: {name}. Install dependencies with: python -m pip install torch transformers accelerate"
            ) from exc

        if device == "auto":
            if torch.cuda.is_available():
                selected = "cuda"
            elif hasattr(torch.backends, "mps") and torch.backends.mps.is_available():
                selected = "mps"
            else:
                selected = "cpu"
        else:
            selected = device

        print(f"[python-openai-server] loading model={model_id} device={selected}", file=sys.stderr, flush=True)
        print("[python-openai-server] loading tokenizer ...", file=sys.stderr, flush=True)
        tokenizer = AutoTokenizer.from_pretrained(model_id, trust_remote_code=True)
        print("[python-openai-server] tokenizer ready", file=sys.stderr, flush=True)
        kwargs = {"trust_remote_code": True, "low_cpu_mem_usage": True}
        if selected == "cuda":
            kwargs["device_map"] = "auto"
            kwargs["torch_dtype"] = torch.bfloat16
        elif selected == "mps":
            kwargs["torch_dtype"] = torch.bfloat16
        elif selected == "cpu":
            kwargs["torch_dtype"] = torch.float32
        print("[python-openai-server] loading model weights ...", file=sys.stderr, flush=True)
        model = AutoModelForCausalLM.from_pretrained(model_id, **kwargs)
        print("[python-openai-server] model weights ready", file=sys.stderr, flush=True)
        if selected in {"mps", "cpu"}:
            print(f"[python-openai-server] moving model to {selected} ...", file=sys.stderr, flush=True)
            model = model.to(selected)
        model.eval()
        MODEL = model
        TOKENIZER = tokenizer
        DEVICE = selected
        MODEL_ID = model_id
        print("[python-openai-server] model ready", file=sys.stderr, flush=True)
        return TOKENIZER, MODEL, DEVICE


def normalize_messages(messages):
    out = []
    for msg in messages or []:
        role = msg.get("role", "user")
        content = msg.get("content", "")
        if content is None:
            content = ""
        if not isinstance(content, str):
            content = json.dumps(content, ensure_ascii=False)
        if role == "tool":
            role = "user"
            content = "工具返回：" + content
        if role in {"system", "user", "assistant"}:
            out.append({"role": role, "content": content})
    return out


def build_prompt(tokenizer, messages):
    normalized = normalize_messages(messages)
    if hasattr(tokenizer, "apply_chat_template"):
        return tokenizer.apply_chat_template(normalized, tokenize=False, add_generation_prompt=True)
    parts = []
    for msg in normalized:
        parts.append(f"{msg['role']}: {msg['content']}")
    parts.append("assistant:")
    return "\n".join(parts)


def resolve_model_id(requested):
    configured = os.environ.get("PY_OPENAI_MODEL", "Qwen/Qwen2.5-3B-Instruct")
    aliases = {
        "qwen1.5-1.8b-chat": "Qwen/Qwen1.5-1.8B-Chat",
        "qwen2.5-3b-instruct": "Qwen/Qwen2.5-3B-Instruct",
        "qwen2.5-7b-instruct": "Qwen/Qwen2.5-7B-Instruct",
        "qwen2.5-14b-instruct": "Qwen/Qwen2.5-14B-Instruct",
    }
    if requested in aliases:
        return aliases[requested]
    if requested and "/" in requested:
        return requested
    return configured


def generate_text(req):
    import torch

    model_id = resolve_model_id(req.get("model"))
    tokenizer, model, device = load_model(model_id, os.environ.get("PY_OPENAI_DEVICE", "auto"))
    prompt = build_prompt(tokenizer, req.get("messages", []))
    inputs = tokenizer(prompt, return_tensors="pt")
    if device != "cuda" or not hasattr(model, "hf_device_map"):
        inputs = inputs.to(device)
    max_tokens = int(req.get("max_tokens") or req.get("max_new_tokens") or os.environ.get("PY_OPENAI_MAX_TOKENS", "512"))
    temperature = float(req.get("temperature") if req.get("temperature") is not None else 0.7)
    top_p = float(req.get("top_p") if req.get("top_p") is not None else 0.9)
    do_sample = temperature > 0
    generate_kwargs = {
        "max_new_tokens": max_tokens,
        "do_sample": do_sample,
        "temperature": temperature if do_sample else None,
        "top_p": top_p if do_sample else None,
        "eos_token_id": tokenizer.eos_token_id,
        "pad_token_id": tokenizer.eos_token_id,
    }
    generate_kwargs = {k: v for k, v in generate_kwargs.items() if v is not None}
    with torch.no_grad():
        output = model.generate(**inputs, **generate_kwargs)
    new_tokens = output[0][inputs["input_ids"].shape[1]:]
    text = tokenizer.decode(new_tokens, skip_special_tokens=True).strip()
    usage = {
        "prompt_tokens": int(inputs["input_ids"].shape[1]),
        "completion_tokens": int(new_tokens.shape[0]),
        "total_tokens": int(inputs["input_ids"].shape[1] + new_tokens.shape[0]),
    }
    return text, usage, model_id


def response_payload(model_id, text, usage):
    return {
        "id": "chatcmpl-" + uuid.uuid4().hex,
        "object": "chat.completion",
        "created": int(time.time()),
        "model": model_id,
        "choices": [
            {
                "index": 0,
                "message": {"role": "assistant", "content": text},
                "finish_reason": "stop",
            }
        ],
        "usage": usage,
    }


class Handler(BaseHTTPRequestHandler):
    server_version = "python-openai-server/0.1"

    def log_message(self, fmt, *args):
        print("[python-openai-server] " + fmt % args, file=sys.stderr, flush=True)

    def send_json(self, status, payload):
        raw = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def do_GET(self):
        if self.path in {"/", "/healthz", "/v1/models"}:
            configured_model = os.environ.get("PY_OPENAI_MODEL", "Qwen/Qwen2.5-3B-Instruct")
            loaded_model = MODEL_ID or ""
            device = DEVICE or os.environ.get("PY_OPENAI_DEVICE", "auto")
            if self.path == "/v1/models":
                self.send_json(200, {
                    "object": "list",
                    "service": "python-openai-server",
                    "configured_model": configured_model,
                    "loaded_model": loaded_model,
                    "device": device,
                    "data": [{"id": loaded_model or configured_model, "object": "model", "owned_by": "local-transformers"}],
                })
            else:
                self.send_json(200, {
                    "ok": True,
                    "service": "python-openai-server",
                    "configured_model": configured_model,
                    "loaded_model": loaded_model,
                    "device": device,
                })
            return
        self.send_json(404, {"error": {"message": "not found"}})

    def do_POST(self):
        if self.path != "/v1/chat/completions":
            self.send_json(404, {"error": {"message": "not found"}})
            return
        try:
            length = int(self.headers.get("Content-Length", "0"))
            req = json.loads(self.rfile.read(length).decode("utf-8") or "{}")
            text, usage, model_id = generate_text(req)
            if req.get("stream"):
                self.stream_response(model_id, text, usage)
            else:
                self.send_json(200, response_payload(model_id, text, usage))
        except Exception as exc:
            self.send_json(500, {"error": {"message": str(exc), "type": exc.__class__.__name__}})

    def stream_response(self, model_id, text, usage):
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream; charset=utf-8")
        self.send_header("Cache-Control", "no-cache")
        self.send_header("Connection", "keep-alive")
        self.end_headers()
        for ch in text:
            chunk = {
                "id": "chatcmpl-" + uuid.uuid4().hex,
                "object": "chat.completion.chunk",
                "created": int(time.time()),
                "model": model_id,
                "choices": [{"index": 0, "delta": {"content": ch}, "finish_reason": None}],
            }
            self.wfile.write(("data: " + json.dumps(chunk, ensure_ascii=False) + "\n\n").encode("utf-8"))
            self.wfile.flush()
        done = {
            "id": "chatcmpl-" + uuid.uuid4().hex,
            "object": "chat.completion.chunk",
            "created": int(time.time()),
            "model": model_id,
            "choices": [{"index": 0, "delta": {}, "finish_reason": "stop"}],
            "usage": usage,
        }
        self.wfile.write(("data: " + json.dumps(done, ensure_ascii=False) + "\n\n").encode("utf-8"))
        self.wfile.write(b"data: [DONE]\n\n")
        self.wfile.flush()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--host", default=os.environ.get("PY_OPENAI_HOST", "127.0.0.1"))
    parser.add_argument("--port", type=int, default=int(os.environ.get("PY_OPENAI_PORT", "18080")))
    parser.add_argument("--model", default=os.environ.get("PY_OPENAI_MODEL", "Qwen/Qwen2.5-3B-Instruct"))
    parser.add_argument("--device", default=os.environ.get("PY_OPENAI_DEVICE", "auto"), choices=["auto", "cuda", "mps", "cpu"])
    parser.add_argument("--lazy", action="store_true", default=os.environ.get("PY_OPENAI_LAZY", "0") == "1")
    args = parser.parse_args()
    os.environ["PY_OPENAI_MODEL"] = args.model
    os.environ["PY_OPENAI_DEVICE"] = args.device
    if not args.lazy:
        load_model(args.model, args.device)
    addr = (args.host, args.port)
    print(f"[python-openai-server] listening on http://{args.host}:{args.port}/v1", file=sys.stderr, flush=True)
    ThreadingHTTPServer(addr, Handler).serve_forever()


if __name__ == "__main__":
    main()
