//
// util.js
//
var Serf = Serf || {};

(function () {

    // calls the given function if the given classname is found
    function runIfClassNamePresent(selector, initFunction) {
        var elms = document.getElementsByClassName(selector);
        if (elms.length > 0) {
            initFunction();
        }
    }

    Serf.Util = {};
    Serf.Util.runIfClassNamePresent = runIfClassNamePresent;

})();