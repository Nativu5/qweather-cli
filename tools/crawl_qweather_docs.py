#!/usr/bin/env python3
"""Download QWeather documentation into a local, searchable Markdown corpus.

The crawler intentionally uses QWeather's published sitemap instead of walking
arbitrary links. It writes one Markdown file per documentation page plus a JSON
index containing breadcrumbs, links, and API endpoints found on each page.
"""

from __future__ import annotations

import argparse
import hashlib
import http.cookiejar
import json
import re
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
import xml.etree.ElementTree as ET
from dataclasses import asdict, dataclass
from datetime import datetime, timezone
from email.message import Message
from pathlib import Path
from typing import Iterable, Sequence

try:
    from bs4 import BeautifulSoup, Tag
    from markdownify import markdownify
except ModuleNotFoundError as exc:  # pragma: no cover - exercised by users
    print(
        "Missing documentation crawler dependencies. Install them with:\n"
        "  python3 -m pip install -r tools/requirements-docs.txt",
        file=sys.stderr,
    )
    raise SystemExit(2) from exc


BASE_URL = "https://dev.qweather.com"
DEFAULT_SITEMAP = f"{BASE_URL}/zh/sitemap.xml"
DEFAULT_OUTPUT = Path(".cache/qweather-docs")
USER_AGENT = (
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
    "AppleWebKit/537.36 (KHTML, like Gecko) "
    "Chrome/138.0.0.0 Safari/537.36"
)
REQUEST_HEADERS = {
    "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
    "Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
    "Sec-Fetch-Dest": "document",
    "Sec-Fetch-Mode": "navigate",
    "Sec-Fetch-Site": "none",
    "Upgrade-Insecure-Requests": "1",
    "User-Agent": USER_AGENT,
}
HTTP_METHOD_RE = re.compile(
    r"^(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)\s+(\S+)$", re.IGNORECASE
)
BLANK_LINES_RE = re.compile(r"\n{3,}")


@dataclass(frozen=True)
class Endpoint:
    method: str
    path: str


@dataclass(frozen=True)
class PageRecord:
    title: str
    url: str
    canonical_url: str
    description: str
    breadcrumbs: list[str]
    endpoints: list[Endpoint]
    links: list[str]
    output_file: str
    content_sha256: str
    last_modified: str


class CrawlError(RuntimeError):
    """A page could not be fetched or parsed as QWeather documentation."""


def parse_args(argv: Sequence[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Build a structured local corpus from the QWeather docs sitemap."
    )
    parser.add_argument(
        "--output",
        type=Path,
        default=DEFAULT_OUTPUT,
        help=f"output directory (default: {DEFAULT_OUTPUT})",
    )
    parser.add_argument(
        "--sitemap",
        default=DEFAULT_SITEMAP,
        help=f"sitemap URL (default: {DEFAULT_SITEMAP})",
    )
    parser.add_argument(
        "--prefix",
        action="append",
        default=[],
        metavar="PATH",
        help=(
            "only download URL paths below PATH; repeatable. "
            "Example: --prefix /docs/api/"
        ),
    )
    parser.add_argument(
        "--include-deprecated",
        action="store_true",
        help="include pages below /docs/deprecated/",
    )
    parser.add_argument(
        "--delay",
        type=float,
        default=0.25,
        help="minimum delay between page requests in seconds (default: 0.25)",
    )
    parser.add_argument(
        "--timeout",
        type=float,
        default=30.0,
        help="per-request timeout in seconds (default: 30)",
    )
    parser.add_argument(
        "--retries",
        type=int,
        default=3,
        help="request attempts for transient failures (default: 3)",
    )
    parser.add_argument(
        "--max-pages",
        type=int,
        default=0,
        help="download at most this many pages; 0 means all matching pages",
    )
    args = parser.parse_args(argv)
    if args.delay < 0:
        parser.error("--delay must be non-negative")
    if args.timeout <= 0:
        parser.error("--timeout must be positive")
    if args.retries <= 0:
        parser.error("--retries must be positive")
    if args.max_pages < 0:
        parser.error("--max-pages must be non-negative")
    args.prefix = [normalize_prefix(prefix, parser) for prefix in args.prefix]
    return args


def normalize_prefix(prefix: str, parser: argparse.ArgumentParser) -> str:
    parsed = urllib.parse.urlparse(prefix)
    if parsed.scheme or parsed.netloc:
        parser.error("--prefix must be a URL path, not a full URL")
    if not prefix.startswith("/docs/"):
        parser.error("--prefix must start with /docs/")
    return prefix if prefix.endswith("/") else f"{prefix}/"


def make_opener() -> urllib.request.OpenerDirector:
    cookie_jar = http.cookiejar.CookieJar()
    return urllib.request.build_opener(
        urllib.request.HTTPCookieProcessor(cookie_jar),
    )


def fetch_text(
    opener: urllib.request.OpenerDirector,
    url: str,
    *,
    timeout: float,
    retries: int,
    accept: str | None = None,
) -> tuple[str, Message]:
    headers = dict(REQUEST_HEADERS)
    if accept:
        headers["Accept"] = accept

    last_error: BaseException | None = None
    for attempt in range(1, retries + 1):
        request = urllib.request.Request(url, headers=headers, method="GET")
        try:
            with opener.open(request, timeout=timeout) as response:
                body = response.read()
                charset = response.headers.get_content_charset() or "utf-8"
                text = body.decode(charset, errors="replace")
                if "<title>Forbidden - QWeather</title>" not in text:
                    return text, response.headers
                last_error = CrawlError("QWeather CDN returned its forbidden page")
        except urllib.error.HTTPError as exc:
            last_error = exc
            if exc.code not in {403, 408, 425, 429, 500, 502, 503, 504}:
                break
        except (TimeoutError, urllib.error.URLError) as exc:
            last_error = exc

        if attempt < retries:
            time.sleep(min(2 ** (attempt - 1), 4))

    detail = f": {last_error}" if last_error else ""
    raise CrawlError(f"failed to fetch {url} after {retries} attempt(s){detail}")


def discover_doc_urls(
    xml_text: str,
    *,
    prefixes: Sequence[str],
    include_deprecated: bool,
) -> list[str]:
    try:
        root = ET.fromstring(xml_text)
    except ET.ParseError as exc:
        raise CrawlError(f"invalid sitemap XML: {exc}") from exc

    urls: list[str] = []
    for element in root.iter():
        if element.tag.rsplit("}", 1)[-1] != "loc" or not element.text:
            continue
        url = canonicalize_url(element.text.strip())
        parsed = urllib.parse.urlparse(url)
        if parsed.scheme != "https" or parsed.netloc != "dev.qweather.com":
            continue
        if not parsed.path.startswith("/docs/"):
            continue
        if not include_deprecated and parsed.path.startswith("/docs/deprecated/"):
            continue
        if prefixes and not any(parsed.path.startswith(prefix) for prefix in prefixes):
            continue
        urls.append(url)
    return sorted(set(urls), key=lambda item: urllib.parse.urlparse(item).path)


def canonicalize_url(url: str) -> str:
    parsed = urllib.parse.urlsplit(url)
    path = parsed.path or "/"
    if not path.endswith("/") and "." not in path.rsplit("/", 1)[-1]:
        path = f"{path}/"
    return urllib.parse.urlunsplit((parsed.scheme, parsed.netloc, path, parsed.query, ""))


def parse_page(html: str, url: str, headers: Message, output_root: Path) -> PageRecord:
    soup = BeautifulSoup(html, "html.parser")
    article = soup.select_one("article.article") or soup.select_one("main")
    content = article.select_one("section.doc-content") if article else None
    if article is not None and content is None:
        # The /docs/ landing page uses cards directly inside <main> instead of
        # the article wrapper shared by individual documentation pages.
        content = article
    if article is None or content is None:
        page_title = soup.title.get_text(" ", strip=True) if soup.title else "unknown page"
        raise CrawlError(f"{url} is not a documentation page ({page_title})")

    headline = article.select_one(".article-headline h1") or article.select_one("h1")
    title = headline.get_text(" ", strip=True) if headline else "Untitled"
    description_tag = soup.select_one('meta[name="description"]')
    description = description_tag.get("content", "").strip() if description_tag else ""
    canonical_tag = soup.select_one('link[rel="canonical"]')
    canonical_url = canonicalize_url(
        urllib.parse.urljoin(url, canonical_tag.get("href", url))
        if canonical_tag
        else url
    )

    breadcrumbs = [
        link.get_text(" ", strip=True)
        for link in soup.select("section.doc-bc a")
        if link.get_text(" ", strip=True)
    ]
    endpoints = extract_endpoints(article)
    links = extract_links(content, url)
    prepare_content_for_markdown(content, url)
    body_markdown = normalize_markdown(markdownify(str(content), heading_style="ATX"))

    relative_output = page_output_path(url)
    output_file = output_root / relative_output
    markdown = render_page_markdown(
        title=title,
        url=canonical_url,
        description=description,
        breadcrumbs=breadcrumbs,
        endpoints=endpoints,
        body=body_markdown,
    )
    write_text(output_file, markdown)

    return PageRecord(
        title=title,
        url=url,
        canonical_url=canonical_url,
        description=description,
        breadcrumbs=breadcrumbs,
        endpoints=endpoints,
        links=links,
        output_file=relative_output.as_posix(),
        content_sha256=hashlib.sha256(markdown.encode("utf-8")).hexdigest(),
        last_modified=headers.get("Last-Modified", ""),
    )


def extract_endpoints(article: Tag) -> list[Endpoint]:
    endpoints: list[Endpoint] = []
    for code in article.select(".request-url code"):
        match = HTTP_METHOD_RE.match(code.get_text(" ", strip=True))
        if match:
            endpoints.append(Endpoint(method=match.group(1).upper(), path=match.group(2)))
    return endpoints


def extract_links(content: Tag, page_url: str) -> list[str]:
    links: set[str] = set()
    for link in content.select("a[href]"):
        href = link.get("href", "").strip()
        if not href or href.startswith(("mailto:", "javascript:")):
            continue
        links.add(urllib.parse.urljoin(page_url, href))
    return sorted(links)


def prepare_content_for_markdown(content: Tag, page_url: str) -> None:
    for permalink in content.select('a[aria-label="永久链接"]'):
        permalink.decompose()
    for link in content.select("a[href]"):
        href = link.get("href", "").strip()
        if href and not href.startswith(("#", "mailto:", "javascript:")):
            link["href"] = urllib.parse.urljoin(page_url, href)
    for image in content.select("img[src]"):
        image["src"] = urllib.parse.urljoin(page_url, image.get("src", ""))


def page_output_path(url: str) -> Path:
    parsed = urllib.parse.urlparse(url)
    parts = [part for part in parsed.path.split("/") if part]
    if not parts or parts[0] != "docs":
        raise CrawlError(f"cannot map non-documentation URL to a file: {url}")
    if len(parts) == 1:
        return Path("pages/docs/index.md")
    return Path("pages", *parts[:-1], f"{parts[-1]}.md")


def normalize_markdown(markdown: str) -> str:
    lines = [line.rstrip() for line in markdown.splitlines()]
    normalized = "\n".join(lines).strip()
    return f"{BLANK_LINES_RE.sub(chr(10) * 2, normalized)}\n"


def render_page_markdown(
    *,
    title: str,
    url: str,
    description: str,
    breadcrumbs: Sequence[str],
    endpoints: Sequence[Endpoint],
    body: str,
) -> str:
    lines = [f"# {title}", "", f"Source: {url}"]
    if description:
        lines.extend(["", description])
    if breadcrumbs:
        lines.extend(["", f"Breadcrumbs: {' > '.join(breadcrumbs)}"])
    if endpoints:
        lines.extend(["", "Endpoints:"])
        lines.extend(f"- `{item.method} {item.path}`" for item in endpoints)
    lines.extend(["", body.rstrip(), ""])
    return "\n".join(lines)


def record_to_dict(record: PageRecord) -> dict[str, object]:
    value = asdict(record)
    value["endpoints"] = [asdict(endpoint) for endpoint in record.endpoints]
    return value


def build_tree(records: Iterable[PageRecord]) -> dict[str, object]:
    root: dict[str, object] = {}
    for record in records:
        parts = [part for part in urllib.parse.urlparse(record.url).path.split("/") if part]
        cursor = root
        for part in parts:
            node = cursor.setdefault(part, {"_children": {}})
            assert isinstance(node, dict)
            cursor = node["_children"]  # type: ignore[assignment]
            assert isinstance(cursor, dict)
        node["_title"] = record.title
        node["_url"] = record.canonical_url
    return root


def write_outputs(
    output: Path,
    records: Sequence[PageRecord],
    failures: Sequence[str],
    source_sitemap: str,
) -> None:
    generated_at = datetime.now(timezone.utc).isoformat(timespec="seconds")
    index = {
        "source": source_sitemap,
        "generated_at": generated_at,
        "page_count": len(records),
        "endpoint_count": sum(len(record.endpoints) for record in records),
        "pages": [record_to_dict(record) for record in records],
        "failures": list(failures),
    }
    write_json(output / "index.json", index)
    write_json(output / "tree.json", build_tree(records))

    category_counts: dict[str, int] = {}
    for record in records:
        parts = [part for part in urllib.parse.urlparse(record.url).path.split("/") if part]
        category = parts[1] if len(parts) > 1 else "docs"
        category_counts[category] = category_counts.get(category, 0) + 1

    summary = [
        "# QWeather documentation corpus",
        "",
        f"Generated: {generated_at}",
        f"Pages: {len(records)}",
        f"API endpoints discovered: {index['endpoint_count']}",
        "",
        "## Categories",
        "",
    ]
    summary.extend(f"- {name}: {count}" for name, count in sorted(category_counts.items()))
    if failures:
        summary.extend(["", "## Failures", ""])
        summary.extend(f"- {failure}" for failure in failures)
    summary.extend(
        [
            "",
            "Use `index.json` for metadata and endpoint search, `tree.json` for the",
            "URL hierarchy, and `pages/` for the normalized Markdown corpus.",
            "",
        ]
    )
    write_text(output / "README.md", "\n".join(summary))


def write_json(path: Path, value: object) -> None:
    write_text(path, f"{json.dumps(value, ensure_ascii=False, indent=2)}\n")


def write_text(path: Path, value: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_suffix(f"{path.suffix}.tmp")
    temporary.write_text(value, encoding="utf-8")
    temporary.replace(path)


def crawl(args: argparse.Namespace) -> int:
    output = args.output.resolve()
    opener = make_opener()
    print(f"Fetching sitemap: {args.sitemap}")
    sitemap, _ = fetch_text(
        opener,
        args.sitemap,
        timeout=args.timeout,
        retries=args.retries,
        accept="application/xml,text/xml;q=0.9,*/*;q=0.8",
    )
    urls = discover_doc_urls(
        sitemap,
        prefixes=args.prefix,
        include_deprecated=args.include_deprecated,
    )
    if args.max_pages:
        urls = urls[: args.max_pages]
    if not urls:
        raise CrawlError("the sitemap did not contain any matching documentation pages")

    records: list[PageRecord] = []
    failures: list[str] = []
    print(f"Downloading {len(urls)} page(s) into {output}")
    for index, url in enumerate(urls, start=1):
        try:
            html, headers = fetch_text(
                opener,
                url,
                timeout=args.timeout,
                retries=args.retries,
            )
            record = parse_page(html, url, headers, output)
            records.append(record)
            print(f"[{index:>3}/{len(urls)}] {record.title}")
        except CrawlError as exc:
            failures.append(str(exc))
            print(f"[{index:>3}/{len(urls)}] ERROR: {exc}", file=sys.stderr)
        if index < len(urls) and args.delay:
            time.sleep(args.delay)

    write_outputs(output, records, failures, args.sitemap)
    print(
        f"Done: {len(records)} page(s), "
        f"{sum(len(record.endpoints) for record in records)} endpoint(s), "
        f"{len(failures)} failure(s)"
    )
    return 1 if failures else 0


def main(argv: Sequence[str] | None = None) -> int:
    try:
        return crawl(parse_args(argv))
    except (CrawlError, OSError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
