#!/usr/bin/env bash
# Example glob groups for commonly problematic platforms, pushed through the
# web UI's form endpoint (there is no separate JSON API).
#
# Usage: NETHADONE_URL=http://<router>:3000 scripts/example-globs.sh
# Comment out the lines you don't want. Re-running is harmless: existing names
# come back as 422 "does it already exist?".
#
# The raw request is just a form post:
#   curl -i http://<router>:3000/globs \
#     --data-urlencode name=reddit \
#     --data-urlencode description="Reddit" \
#     --data-urlencode glob='*reddit*'
#   -> 303 on success, 422 (HTML page with the error) on failure.
# Delete one with:
#   curl -i http://<router>:3000/globs/delete --data-urlencode name=reddit
#
# Glob notes:
# - sqlite GLOB syntax, one pattern per group (no {a,b}), so a
#   platform with unrelated domain names needs several groups. Each group is
#   throttled on its own.
# - Domains and globs are stored lowercased without the trailing dot
#   ("www.reddit.com"). An IP is stored under the queried name and every CNAME
#   in its chain, so a glob on the name users type matches CDN-backed sites.
# - "*reddit.com" also matches the bare domain, unlike "*.reddit.com", at the
#   cost of matching e.g. "notreddit.com".
# - Video CDN domains (googlevideo, nflxvideo, ttvnw, ...) carry the bulk of the
#   bytes, so they matter more than the main site's domain.
# - Every matching IP becomes a throttler rule; very broad groups on a busy
#   network push toward the ~1000 rule limit (see AGENTS.md known issues).

URL=${NETHADONE_URL:-http://localhost:3000}

add() { # name description glob
	code=$(curl -s -o /dev/null -w '%{http_code}' "$URL/globs" \
		--data-urlencode "name=$1" --data-urlencode "description=$2" --data-urlencode "glob=$3")
	case $code in
	303) echo "added   $1 ($3)" ;;
	422) echo "skipped $1 ($3): invalid or already exists" ;;
	*) echo "failed  $1 ($3): HTTP $code" >&2 ;;
	esac
}

# Short video
add tiktok      "TikTok (site, API, CDNs)"            '*tiktok*'          # tiktok.com, tiktokv.com, tiktokcdn(-us).com
add youtube     "YouTube site"                        '*youtube.com'
add youtube-cdn "YouTube video CDN"                   '*googlevideo.com'
add youtube-img "YouTube thumbnails"                  '*ytimg.com'

# Social feeds
add instagram   "Instagram (site + cdninstagram.com)" '*instagram.com'
add facebook    "Facebook"                            '*facebook.com'
add fbcdn       "Meta CDN (Facebook + Instagram media)" '*fbcdn.net'
add snapchat    "Snapchat"                            '*snapchat.com'
add snap-cdn    "Snapchat CDN"                        '*sc-cdn.net'
add twitter     "X/Twitter media"                     '*twimg.com'
add twitter-web "Twitter site"                        '*twitter.com'
add x           "X site (subdomains only)"            '*.x.com'           # '*x.com' would also match netflix.com
add reddit      "Reddit (site, static, media)"        '*reddit*'          # reddit.com, redditmedia.com, redditstatic.com
add reddit-img  "Reddit images/video"                 '*redd.it'          # i.redd.it, v.redd.it
add pinterest   "Pinterest"                           '*pinterest.com'
add pinimg      "Pinterest images"                    '*pinimg.com'

# Streaming
add twitch      "Twitch site"                         '*twitch.tv'
add twitch-cdn  "Twitch video"                        '*ttvnw.net'
add netflix     "Netflix site"                        '*netflix.com'
add netflix-cdn "Netflix video/images"                '*nflx*'            # nflxvideo.net, nflxso.net, nflximg.net, nflxext.com

# Games
add roblox      "Roblox"                              '*roblox.com'
add roblox-cdn  "Roblox CDN"                          '*rbxcdn.com'
