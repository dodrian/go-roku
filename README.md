# go-roku
This is a server written in Go to browse items in a Jellyfin library and launch them on a Roku device with a single click.

# setup

Ensure your Roku and your Jellyfin server have either a DNS name or a reserved network IP address.

Edit `docker-compose.yml` to add all enviroment variables.  URLs should include http\[s\]:// but not a trailing /
Expose a different port if 8000 is not desired/available.

Run `docker-compose up -d --build` to launch server as a background process.


# Todo:

* Pull down bootstrap CSS dependency when 
* Add a favicon so it looks nicer as a ad-hoc phone app
* Fix image aspect ratios?
