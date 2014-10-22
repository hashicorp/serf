#-------------------------------------------------------------------------
# Configure Middleman
#-------------------------------------------------------------------------

set :base_url, "https://www.serfdom.io/"

activate :hashicorp do |h|
  h.version      = '0.6.3'
  h.bintray_repo = 'mitchellh/serf'
  h.bintray_user = 'mitchellh'
  h.bintray_key  = ENV['BINTRAY_API_KEY']

  # Currently, Serf builds are not prefixed with serf_*
  h.bintray_prefixed = false
end

helpers do
  # This helps by setting the "active" class for sidebar nav elements
  # if the YAML frontmatter matches the expected value.
  def sidebar_current(expected)
    current = current_page.data.sidebar_current
    if current.start_with?(expected)
      return " class=\"active\""
    else
      return ""
    end
  end
end
