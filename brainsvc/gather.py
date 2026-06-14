from __future__ import annotations

import httpx

UA = {"User-Agent": "bourse/1.0"}

def quote(symbol: str) -> dict:
    r = httpx.get(
        "https://query1.finance.yahoo.com/v8/finance/chart/" + symbol,
        headers=UA, timeout=15,
    )
    r.raise_for_status()
    res = r.json()["chart"]["result"][0]["meta"]
    price = res["regularMarketPrice"]
    prev = res.get("previousClose", price)
    return {"price": price, "change_pct": (price - prev) / prev * 100 if prev else 0.0}

def headlines(symbol: str, n: int = 6) -> list[str]:
    r = httpx.get(
        "https://news.google.com/rss/search",
        params={"q": symbol + " stock", "hl": "en-US", "gl": "US", "ceid": "US:en"},
        headers=UA, timeout=15,
    )
    import re
    titles = re.findall(r"<title>(.*?)</title>", r.text)[1:]  # skip feed title
    return [t.replace("&amp;", "&") for t in titles[:n]]

def _sentiment(hs: list[str]) -> list[float]:
    from brainsvc.tier0.sentiment import score
    return score(hs)

def _rank(sym: str, hs: list[str], k: int) -> list[str]:
    from brainsvc.tier0.rank import top_k
    return top_k(sym, hs, k)

def build_context(watchlist: list[str]) -> tuple[str, dict]:
    """Return (prompt-context string, {symbol: ref_price})."""
    buf, ref = [], {}
    for s in watchlist:
        try:
            q = quote(s)
        except Exception:
            continue
        ref[s] = q["price"]
        buf.append(f"\n## {s}\nprice {q['price']:.2f} ({q['change_pct']:+.2f}%)")
        raw = headlines(s, 6)
        hs = _rank(s, raw, 4)
        sents = _sentiment(hs)
        for h, sv in zip(hs, sents):
            buf.append(f"- {h}  [sentiment {sv:+.2f}]")
    return "\n".join(buf), ref
