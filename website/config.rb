#-------------------------------------------------------------------------
# Configure Middleman
#-------------------------------------------------------------------------

set :base_url, "https://www.serfdom.io/"

activate :hashicorp do |h|
  h.version      = '0.6.3'
  h.bintray_repo = 'mitchellh/serf'
  h.bintray_user = 'mitchellh'
  h.bintray_key  = ENV['BINTRAY_API_KEY']
end
