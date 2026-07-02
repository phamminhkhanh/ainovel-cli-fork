#!/usr/bin/env python3
"""
MyNovel.net Trend Analyzer v3 — Clean Build
Multi-page scraping + actionable insight generation.

Usage:
    python mynovel_analyzer.py              # default 5 pages (~100 books)
    python mynovel_analyzer.py --pages 10   # scrape 10 pages (~200 books)

Requirements: pip install requests beautifulsoup4
"""

import requests
from bs4 import BeautifulSoup
import json
import re
import sys
import time
from collections import Counter
from datetime import datetime
from itertools import combinations
from statistics import median

# ── Config ────────────────────────────────────────────────────────────

BASE_URL = "https://mynovel.net/en/genre/all?sortBy=popular"
HEADERS = {
    "User-Agent": (
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) "
        "AppleWebKit/537.36 (KHTML, like Gecko) "
        "Chrome/126.0 Safari/537.36"
    ),
    "Accept": "text/html,application/xhtml+xml",
    "Accept-Language": "en-US,en;q=0.5",
}
REQUEST_DELAY = 1.5  # seconds between page requests

_TS = datetime.now().strftime("%Y%m%d_%H%M")
OUTPUT_JSON = f"mynovel_{_TS}.json"
OUTPUT_TXT = f"mynovel_{_TS}.txt"

# Tags too generic to be useful in combo analysis
GENERIC_TAGS = {"Contemporary Romance", "Romance", "Erotic Romance"}

BAR = "#"


# ── Utilities ─────────────────────────────────────────────────────────

def parse_number(text: str) -> int:
    """Parse '2.4M', '537.6K', '12,200' → int."""
    if not text:
        return 0
    text = text.strip().lower().replace(",", "")
    try:
        if "m" in text:
            return int(float(text.replace("m", "")) * 1_000_000)
        if "k" in text:
            return int(float(text.replace("k", "")) * 1_000)
        return int(float(text))
    except ValueError:
        return 0


def hbar(value: float, max_val: float, width: int = 25) -> str:
    """Horizontal ASCII bar."""
    if max_val <= 0:
        return ""
    return BAR * int((value / max_val) * width)


def fmt_reads(n: int) -> str:
    """Format reads: 2400000 → '2.4M', 537600 → '537.6K'."""
    if n >= 1_000_000:
        return f"{n / 1_000_000:.1f}M"
    if n >= 1_000:
        return f"{n / 1_000:.1f}K"
    return str(n)


# ── Scraper ───────────────────────────────────────────────────────────

def scrape_page(url: str) -> list[dict]:
    """Scrape a single page of books."""
    resp = requests.get(url, headers=HEADERS, timeout=30)
    resp.raise_for_status()
    soup = BeautifulSoup(resp.text, "html.parser")

    books = []
    for row in soup.find_all("div", class_=re.compile(r"flex flex-row gap-2\.5 w-full")):
        children = [c for c in row.children if hasattr(c, "name")]
        if len(children) < 2:
            continue

        cover_div, info_div = children[0], children[1]

        try:
            # Price
            price = 0.0
            for tid in ("book-price", "mobile-view-book-price"):
                el = cover_div.find("p", {"data-testid": tid})
                if el:
                    pt = el.get_text(strip=True)
                    if "free" not in pt.lower():
                        m = re.search(r"\$?([\d.]+)", pt)
                        if m:
                            price = float(m.group(1))
                    break

            # Title
            title = ""
            title_a = info_div.find("a")
            if title_a:
                tp = title_a.find("p", class_=re.compile(r"font-bold"))
                if tp:
                    title = tp.get_text(strip=True)

            # Author
            author = ""
            for p in info_div.find_all("p"):
                t = p.get_text(strip=True)
                if t.lower().startswith("by "):
                    author = t[3:].strip()
                    break

            # Stats (reads, likes, comments)
            reads = likes = comments = 0
            stats_div = info_div.find("div", class_=re.compile(r"mb-1.*flex.*flex-row.*gap-3\.5"))
            if stats_div:
                vals = []
                for sr in stats_div.find_all("div", class_=re.compile(r"flex flex-row items-center")):
                    vp = sr.find("p", class_=re.compile(r"text-xs.*font-medium"))
                    if vp:
                        vals.append(vp.get_text(strip=True))
                if len(vals) >= 1: reads = parse_number(vals[0])
                if len(vals) >= 2: likes = parse_number(vals[1])
                if len(vals) >= 3: comments = parse_number(vals[2])

            # Tags
            tags = []
            tags_div = info_div.find("div", class_=re.compile(r"flex flex-row gap-1\.5 flex-wrap"))
            if tags_div:
                skip = {"Free", "Completed", "Ongoing", "Bookscription"}
                for a in tags_div.find_all("a"):
                    tag = a.get_text(strip=True)
                    if tag and tag not in skip:
                        tags.append(tag)

            # Status
            status = "Unknown"
            for p in info_div.find_all("p"):
                t = p.get_text(strip=True).lower()
                if t in ("completed", "ongoing"):
                    status = t.title()
                    break

            # Description
            desc = ""
            dd = info_div.find("div", class_=re.compile(r"mb-1\.5"))
            if dd:
                dp = dd.find("p", class_=re.compile(r"text-ellipsis.*line-clamp"))
                if dp:
                    desc = dp.get_text(strip=True)

            if title and reads > 0:
                books.append({
                    "title": title,
                    "author": author,
                    "reads": reads,
                    "likes": likes,
                    "comments": comments,
                    "tags": tags,
                    "price": price,
                    "status": status,
                    "description": desc,
                })
        except (AttributeError, ValueError, IndexError):
            continue

    return books


def scrape_books(pages: int = 5) -> list[dict]:
    """Scrape N pages, dedup by title, sort by reads desc."""
    all_books = []
    seen = set()
    consecutive_empty = 0

    for pg in range(1, pages + 1):
        url = f"{BASE_URL}&page={pg}" if pg > 1 else BASE_URL
        print(f"  [{pg}/{pages}] Fetching...")

        page_books = None
        for attempt in range(3):
            try:
                page_books = scrape_page(url)
                break
            except requests.RequestException as e:
                if attempt < 2:
                    wait = (attempt + 1) * 3
                    print(f"          Retry {attempt+1}/2 in {wait}s... ({e})")
                    time.sleep(wait)
                else:
                    print(f"          FAILED after 3 attempts: {e}")

        if page_books is None:
            break

        added = 0
        for b in page_books:
            if b["title"] not in seen:
                seen.add(b["title"])
                all_books.append(b)
                added += 1

        print(f"          +{added} new (total: {len(all_books)})")

        if added == 0:
            consecutive_empty += 1
            if consecutive_empty >= 2:
                print("  [!] 2 consecutive empty pages — stopping.")
                break
        else:
            consecutive_empty = 0

        if pg < pages:
            time.sleep(REQUEST_DELAY)

    all_books.sort(key=lambda x: x["reads"], reverse=True)
    return all_books


# ── Analysis ──────────────────────────────────────────────────────────

def analyze(books: list[dict]) -> dict:
    n = len(books)
    if n == 0:
        return {}

    # Enrich: engagement ratio
    for b in books:
        b["eng"] = (b["comments"] / b["reads"] * 100) if b["reads"] > 0 else 0

    # ── Tags ──
    all_tags = [t for b in books for t in b["tags"]]
    tag_freq = Counter(all_tags)

    # Meaningful combos: filter out generic tags
    combo_ctr = Counter()
    for b in books:
        specific = sorted(set(b["tags"]) - GENERIC_TAGS)
        for pair in combinations(specific, 2):
            combo_ctr[pair] += 1

    # ── Free vs Paid ──
    free = [b for b in books if b["price"] == 0]
    paid = [b for b in books if b["price"] > 0]
    prices = [b["price"] for b in paid]

    # ── Price tiers ──
    tiers = {"<$2": 0, "$2-$3": 0, "$3+": 0}
    for p in prices:
        if p < 2:     tiers["<$2"] += 1
        elif p < 3:   tiers["$2-$3"] += 1
        else:         tiers["$3+"] += 1

    # ── Status ──
    completed = [b for b in books if b["status"] == "Completed"]
    ongoing   = [b for b in books if b["status"] == "Ongoing"]

    # ── Averages ──
    def avg(lst, key): return sum(b[key] for b in lst) / len(lst) if lst else 0

    # ── Tag engagement matrix ──
    tag_eng = {}
    for tag, cnt in tag_freq.items():
        tagged = [b for b in books if tag in b["tags"]]
        if len(tagged) >= 2:
            tag_eng[tag] = {
                "count": cnt,
                "avg_eng": avg(tagged, "eng"),
                "avg_reads": int(avg(tagged, "reads")),
                "total_reads": sum(b["reads"] for b in tagged),
                "paid_ratio": sum(1 for b in tagged if b["price"] > 0) / len(tagged),
            }

    # ── Blue ocean: few books + high engagement ──
    blue = []
    for tag, d in tag_eng.items():
        if d["count"] <= 5 and d["avg_eng"] > 0.15:
            blue.append((tag, d))
    blue.sort(key=lambda x: x[1]["avg_eng"], reverse=True)

    # ── Revenue estimate ──
    # Conservative range: 0.1% (pessimistic) to 0.5% (optimistic) conversion
    total_paid_reads = sum(b["reads"] for b in paid)
    avg_price = sum(prices) / len(prices) if prices else 0
    est_low = int(total_paid_reads * 0.001 * avg_price)    # 0.1%
    est_mid = int(total_paid_reads * 0.003 * avg_price)    # 0.3%
    est_high = int(total_paid_reads * 0.005 * avg_price)   # 0.5%

    # ── Engagement leaderboard ──
    eng_top = sorted(books, key=lambda x: x["eng"], reverse=True)[:10]

    # ── Reads distribution ──
    all_reads = [b["reads"] for b in books]

    return {
        "n": n,
        "tag_freq": dict(tag_freq.most_common(20)),
        "combos": dict(combo_ctr.most_common(12)),
        "free_n": len(free),
        "paid_n": len(paid),
        "avg_price": round(avg_price, 2),
        "tiers": tiers,
        "completed_n": len(completed),
        "ongoing_n": len(ongoing),
        "free_avg_reads": int(avg(free, "reads")),
        "paid_avg_reads": int(avg(paid, "reads")),
        "free_avg_eng": round(avg(free, "eng"), 3),
        "paid_avg_eng": round(avg(paid, "eng"), 3),
        "comp_avg_eng": round(avg(completed, "eng"), 3),
        "ong_avg_eng": round(avg(ongoing, "eng"), 3),
        "tag_eng": tag_eng,
        "blue": blue,
        "eng_top": eng_top,
        "reads_max": max(all_reads),
        "reads_median": int(median(all_reads)),
        "reads_min": min(all_reads),
        "reads_total": sum(all_reads),
        "est_revenue_low": est_low,
        "est_revenue_mid": est_mid,
        "est_revenue_high": est_high,
    }


# ── Report ────────────────────────────────────────────────────────────

def report(books: list[dict], a: dict) -> str:
    L = []
    W = 78
    now = datetime.now().strftime("%Y-%m-%d %H:%M")
    top20 = books[:20]

    def section(title):
        L.append("")
        L.append(f"  [{title}]{'─' * (W - len(title) - 5)}")

    L.append("=" * W)
    L.append("  MYNOVEL.NET — TREND ANALYSIS REPORT v3")
    L.append(f"  {now}  |  {a['n']} books  |  {fmt_reads(a['reads_total'])} total reads")
    L.append("=" * W)

    # ── [1] TOP 20 ────────────────────────────────────────────────────
    section("1. TOP 20 POPULAR BOOKS")
    L.append(f"  {'#':>2}  {'Title':<34} {'Reads':>8} {'$':>5} {'Cmt':>6} {'Eng':>5}")
    L.append(f"  {'─'*2}  {'─'*34} {'─'*8} {'─'*5} {'─'*6} {'─'*5}")
    mx = top20[0]["reads"]
    for i, b in enumerate(top20, 1):
        p = "FREE" if b["price"] == 0 else f"${b['price']:.0f}"
        L.append(
            f"  {i:>2}. {b['title'][:32]:<34} "
            f"{fmt_reads(b['reads']):>8} {p:>5} {b['comments']:>6,} {b['eng']:>4.1f}%"
        )
        tags_str = ", ".join(b["tags"][:3])
        L.append(f"      {hbar(b['reads'], mx, 20)}  {tags_str}")

    # ── [2] MARKET SIZE ───────────────────────────────────────────────
    section("2. MARKET SNAPSHOT")
    L.append(f"  Total reads (all {a['n']} books):  {a['reads_total']:>14,}")
    L.append(f"  Reads — max / median / min:   {fmt_reads(a['reads_max']):>8} / {fmt_reads(a['reads_median']):>8} / {fmt_reads(a['reads_min']):>8}")
    L.append(f"")
    L.append(f"  Revenue estimate (avg ${a['avg_price']:.2f}/book):")
    L.append(f"    Conservative (0.1%):  ~${a['est_revenue_low']:,}")
    L.append(f"    Moderate    (0.3%):  ~${a['est_revenue_mid']:,}")
    L.append(f"    Optimistic  (0.5%):  ~${a['est_revenue_high']:,}")
    L.append(f"    ⚠  Conversion rate unknown — range shows floor/ceiling.")

    # ── [3] FREE vs PAID ──────────────────────────────────────────────
    section("3. FREE vs PAID")
    total = a["free_n"] + a["paid_n"]
    fp = int(a["free_n"] / total * 100)
    pp = int(a["paid_n"] / total * 100)
    L.append(f"  FREE  {a['free_n']:>3} ({fp}%)  {hbar(fp, 100, 20)}    PAID  {a['paid_n']:>3} ({pp}%)  {hbar(pp, 100, 20)}")
    L.append(f"  Avg price: ${a['avg_price']:.2f}   Tiers: <$2: {a['tiers']['<$2']}  |  $2-$3: {a['tiers']['$2-$3']}  |  $3+: {a['tiers']['$3+']}")
    L.append(f"")
    L.append(f"  {'':30} {'FREE':>10} {'PAID':>10}")
    L.append(f"  {'Avg reads':<30} {fmt_reads(a['free_avg_reads']):>10} {fmt_reads(a['paid_avg_reads']):>10}   FREE gets {a['free_avg_reads']/a['paid_avg_reads']:.1f}x more reads")
    L.append(f"  {'Avg engagement (cmt/reads)':<30} {a['free_avg_eng']:>9.2f}% {a['paid_avg_eng']:>9.2f}%   PAID gets {a['paid_avg_eng']/a['free_avg_eng']:.1f}x more engagement")

    # ── [4] STATUS ────────────────────────────────────────────────────
    section("4. COMPLETED vs ONGOING")
    ts = a["completed_n"] + a["ongoing_n"]
    cp = int(a["completed_n"] / ts * 100) if ts else 0
    L.append(f"  Completed  {a['completed_n']:>3} ({cp}%)  {hbar(cp, 100, 25)}")
    L.append(f"  Ongoing    {a['ongoing_n']:>3} ({100-cp}%)  {hbar(100-cp, 100, 25)}")
    L.append(f"  Engagement:  completed {a['comp_avg_eng']:.2f}%  vs  ongoing {a['ong_avg_eng']:.2f}%")

    # ── [5] TAG FREQUENCY ─────────────────────────────────────────────
    section("5. TAG FREQUENCY (top 15)")
    max_tf = max(a["tag_freq"].values())
    for tag, cnt in list(a["tag_freq"].items())[:15]:
        L.append(f"  {tag:<30} {cnt:>3}  {hbar(cnt, max_tf, 25)}")

    # ── [6] MEANINGFUL TAG COMBOS ─────────────────────────────────────
    section("6. TAG COMBOS (excluding generic Romance/Erotic)")
    if a["combos"]:
        mx_c = max(a["combos"].values())
        for pair, cnt in list(a["combos"].items())[:10]:
            label = f"{pair[0]} + {pair[1]}"
            L.append(f"  {label:<48} {cnt:>3}  {hbar(cnt, mx_c, 15)}")
    else:
        L.append("  Not enough data for meaningful combos.")

    # ── [7] ENGAGEMENT LEADERBOARD ────────────────────────────────────
    section("7. ENGAGEMENT LEADERBOARD (comments ÷ reads)")
    L.append(f"  {'#':>2}  {'Title':<30} {'Eng':>5} {'Cmt':>7} {'Reads':>8} {'$':>5}")
    L.append(f"  {'─'*2}  {'─'*30} {'─'*5} {'─'*7} {'─'*8} {'─'*5}")
    mx_e = a["eng_top"][0]["eng"] if a["eng_top"] else 1
    for i, b in enumerate(a["eng_top"], 1):
        p = "FREE" if b["price"] == 0 else f"${b['price']:.0f}"
        L.append(
            f"  {i:>2}. {b['title'][:28]:<30} "
            f"{b['eng']:>4.1f}% {b['comments']:>7,} {fmt_reads(b['reads']):>8} {p:>5}"
        )

    # ── [8] GENRE OPPORTUNITY MATRIX ──────────────────────────────────
    section("8. GENRE OPPORTUNITY MATRIX")
    L.append(f"  {'Tag':<28} {'Books':>5} {'AvgEng':>6} {'AvgReads':>10} {'Paid%':>5}  Verdict")
    L.append(f"  {'─'*28} {'─'*5} {'─'*6} {'─'*10} {'─'*5}  {'─'*16}")
    sorted_te = sorted(a["tag_eng"].items(), key=lambda x: x[1]["avg_eng"], reverse=True)
    for tag, d in sorted_te[:15]:
        paid_pct = int(d["paid_ratio"] * 100)
        # Classify
        if d["count"] <= 4 and d["avg_eng"] > 0.4:
            verdict = "🔥 BLUE OCEAN"
        elif d["count"] <= 5 and d["avg_eng"] > 0.2:
            verdict = "⚡ Opportunity"
        elif d["avg_eng"] > 0.4 and d["count"] > 5:
            verdict = "✅ Proven hot"
        elif d["avg_eng"] < 0.15:
            verdict = "⚪ Low signal"
        else:
            verdict = "👀 Moderate"
        L.append(
            f"  {tag:<28} {d['count']:>5} {d['avg_eng']:>5.2f}% "
            f"{fmt_reads(d['avg_reads']):>10} {paid_pct:>4}%  {verdict}"
        )

    # ── [9] ACTIONABLE INSIGHTS ───────────────────────────────────────
    section("9. ACTIONABLE INSIGHTS")
    L.append("")

    # Winning formula
    L.append("  WINNING FORMULA (what sells on mynovel.net):")
    top3_tags = list(a["tag_freq"].keys())[:3]
    L.append(f"    Dominant tags: {', '.join(top3_tags)}")
    if a["combos"]:
        top_combo = list(a["combos"].items())[0]
        L.append(f"    Best niche combo: {top_combo[0][0]} + {top_combo[0][1]} ({top_combo[1]} books)")
    L.append(f"    Sweet spot price: ${a['avg_price']:.2f} ($2-$3 tier has {a['tiers']['$2-$3']} books)")
    L.append("")

    # Funnel strategy
    L.append("  MONETIZATION STRATEGY:")
    if a["free_avg_reads"] > a["paid_avg_reads"]:
        ratio_r = a["free_avg_reads"] / a["paid_avg_reads"]
        ratio_e = a["paid_avg_eng"] / a["free_avg_eng"] if a["free_avg_eng"] > 0 else 0
        L.append(f"    FREE gets {ratio_r:.1f}x reads, PAID gets {ratio_e:.1f}x engagement.")
        L.append(f"    → Book 1 FREE (reach) → Book 2+ PAID at ${a['avg_price']:.2f} (convert fans)")
    L.append(f"    → {a['completed_n']}/{a['n']} ({int(a['completed_n']/a['n']*100)}%) are completed → upload finished works, not serials")
    L.append("")

    # Blue ocean
    if a["blue"]:
        L.append("  BLUE OCEAN NICHES (low supply + high engagement):")
        for tag, d in a["blue"][:5]:
            L.append(f"    • {tag}: {d['count']} books, {d['avg_eng']:.2f}% engagement, {fmt_reads(d['avg_reads'])} avg reads")
        L.append("    → First movers in these niches have outsized advantage.")
    L.append("")

    # ainovel-cli
    L.append("  AINOVEL-CLI MAPPING:")
    L.append("    Primary:  style=`romance`  (Contemporary/Erotic/Billionaire)")
    L.append("    Dark mix: style=`suspense` + romance rules  (Dark Romance/Mystery/Mafia)")
    L.append("    Fantasy:  style=`fantasy`  (Romantasy/Werewolf — blue ocean)")
    L.append("")

    # Concrete plan
    L.append("  CONCRETE PLAN — 3 books for mynovel.net:")
    L.append("    #1 [FREE]  Contemporary Romance + Second Chance + Erotic")
    L.append(f"               80-120 chapters, completed, style=`romance`")
    L.append(f"    #2 [${a['avg_price']:.2f}]  Dark Romance + Mystery + Kidnapping")
    L.append(f"               60-100 chapters, completed, style=`suspense`")
    L.append(f"    #3 [${a['avg_price']:.2f}]  Romantasy + Werewolf + Mate Bond")
    L.append(f"               80-120 chapters, completed, style=`fantasy`")
    L.append(f"    Goal: #1 pulls readers → #2/#3 convert to paid")

    L.append("")
    L.append("=" * W)
    L.append(f"  Saved: {OUTPUT_JSON}  |  {OUTPUT_TXT}")
    L.append("=" * W)
    L.append("")

    return "\n".join(L)


# ── Main ──────────────────────────────────────────────────────────────

def main():
    pages = 5
    if "--pages" in sys.argv:
        idx = sys.argv.index("--pages")
        if idx + 1 < len(sys.argv):
            pages = int(sys.argv[idx + 1])

    print("=" * 60)
    print("  MyNovel.net Trend Analyzer v3")
    print(f"  Scraping {pages} pages → analysis → actionable insights")
    print("=" * 60)
    print()

    # 1. Scrape
    books = scrape_books(pages=pages)
    if not books:
        print("[!] No books scraped. Site may have changed.")
        return
    print(f"\n[+] {len(books)} unique books scraped.\n")

    # 2. Analyze
    a = analyze(books)

    # 3. Report
    out = report(books, a)
    print(out)

    # 4. Save
    with open(OUTPUT_JSON, "w", encoding="utf-8") as f:
        json.dump({
            "scraped_at": datetime.now().isoformat(),
            "source": BASE_URL,
            "pages": pages,
            "total_books": len(books),
            "books": books,
            "analysis": {
                "tag_freq": a["tag_freq"],
                "combos": {f"{k[0]} + {k[1]}": v for k, v in a["combos"].items()},
                "free_n": a["free_n"],
                "paid_n": a["paid_n"],
                "avg_price": a["avg_price"],
                "tiers": a["tiers"],
                "completed_n": a["completed_n"],
                "ongoing_n": a["ongoing_n"],
                "reads_total": a["reads_total"],
                "reads_median": a["reads_median"],
                "est_revenue_low": a["est_revenue_low"],
                "est_revenue_mid": a["est_revenue_mid"],
                "est_revenue_high": a["est_revenue_high"],
                "blue_ocean": [(t, d) for t, d in a["blue"]],
            },
        }, f, ensure_ascii=False, indent=2)
    print(f"[+] {OUTPUT_JSON}")

    with open(OUTPUT_TXT, "w", encoding="utf-8") as f:
        f.write(out)
    print(f"[+] {OUTPUT_TXT}")


if __name__ == "__main__":
    main()
