from __future__ import annotations

from functools import lru_cache

# NOTE: Model id and the "1"/"0" supported/not-supported token convention must be
# verified against the MiniCheck model card before the first real model run.
# See: https://huggingface.co/lytang/MiniCheck-Flan-T5-Large
# This is a flagged item — confirm output vocabulary matches the prompt template below.
MODEL = "lytang/MiniCheck-Flan-T5-Large"


@lru_cache(maxsize=1)
def _model():
    from transformers import AutoTokenizer, AutoModelForSeq2SeqLM
    tok = AutoTokenizer.from_pretrained(MODEL)
    mdl = AutoModelForSeq2SeqLM.from_pretrained(MODEL).eval()
    return tok, mdl


def supported(document: str, claim: str) -> float:
    """P(claim is supported by document), 0..1."""
    import torch
    tok, mdl = _model()
    prompt = f"predict: premise: {document} hypothesis: {claim}"
    enc = tok(prompt, return_tensors="pt", truncation=True, max_length=512)
    with torch.no_grad():
        out = mdl.generate(**enc, max_new_tokens=1, output_scores=True,
                           return_dict_in_generate=True)
    # MiniCheck-FT5 emits "1" (supported) / "0" (not). Map first-token prob of "1".
    # FLAGGED: verify that the model card confirms "1"/"0" as the output tokens
    # and that this prompt template matches the expected MiniCheck input format.
    yes_id = tok("1", add_special_tokens=False).input_ids[0]
    logits = out.scores[0][0]
    return torch.softmax(logits, dim=-1)[yes_id].item()
