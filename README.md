# instagramCrawler

instagramCrawler automatically saves images and videos in a local folder structure from public instagram profiles.
Only new Pictures since the last crawl get saved and unix timestamps are added to each file for sorting.

## Legal

Keep in Mind that any usage of this program violates Instagrams Terms of Service:

"8. We prohibit crawling, scraping, caching or otherwise accessing any content on the Service via automated means, including but not limited to, user profiles and photos (except as may be the result of standard search engine protocols or technologies used by a search engine with Instagram's express consent)."
(19.11.2017)

## Usage

Following Launch parameters are available:

c[number] -> limits number of maximum simultaneous http requests.
p[number] -> amount of profile pages (12 Posts each) get saved (default is full profile).
v -> save video flag (default is no video saving)
r[number] -> restart the crawling every [number] seconds. Default is no restarting.
