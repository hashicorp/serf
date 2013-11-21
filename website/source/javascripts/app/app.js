//
// home.js
//

var Serf = (function() {

	function initialize (){
		Serf.Util.runIfClassNamePresent('page-home', initHome);
	}

	function initHome() {
		Serf.Nodes.init(); 
	}
  
  	//api
	return {
		initialize: initialize
  	}

})();