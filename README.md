# go-roku
This is a server written in Go to browse items in a Jellyfin library and launch them on a Roku device with a single click.

# setup

Ensure your Roku and your Jellyfin server have either a DNS name or a reserved network IP address.  Ensure Docker and docker-compose are setup on the server you are deploying to.

Edit `docker-compose.yml` to add all enviroment variables.  URLs should include http\[s\]:// but not a trailing /
Expose a different port if 8000 is not desired/available, and update the GOROKU_URL env variable to match.

Run `docker-compose up -d --build` to launch server as a background process.


# Todo:

* Pull down bootstrap CSS dependency when building docker image (so no calls need to leave local network)
* Add a favicon so it looks nicer as a ad-hoc phone app
* Fix image aspect ratios?
* debug / Fix cors issue  ?
