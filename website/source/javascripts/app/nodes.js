//
// node.js
// animation on the home page
//

var Serf = Serf || {};

(function () {

	//cache node positions
    var nodes = [ 
	{ x: 1431, y: 244 },
	{ x: 1390, y: -18 },
	{ x: 1340, y: 523 },
	{ x: 1118, y: 156 },
	{ x: 1191, y: 67 },
	{ x: 1107, y: 366 },
	{ x: 1100, y: 491 },
	{ x: 1300, y: -52 },
	{ x: 920, y: 151 },
	{ x: 1151, y: 288 },
	{ x: 1200, y: 607 },
	{ x: 968, y: -100 },
	{ x: 1450, y: 415 },
	{ x: 1050, y: 93 },
	{ x: 940, y: 673 },
	{ x: 992, y: 202 },
	{ x: 918, y: 26 },
	{ x: 909, y: 506 },
	{ x: 831, y: -33 },
	{ x: 846, y: 85 },
	{ x: 873, y: -118 },
	{ x: 824, y: 547 },
	{ x: 880, y: 263 },
	{ x: 806, y: 737 },
	{ x: 897, y: 167 },
	{ x: 852, y: -201 },
	{ x: 666, y: 242 },
	{ x: 816, y: 639 },
	{ x: 713, y: 749 },
	{ x: 771, y: 455 },
	{ x: 713, y: 633 },
	{ x: 818, y: 359 },
	{ x: 767, y: -215 },
	{ x: 954, y: 325 },
	{ x: 622, y: 672 },
	{ x: 742, y: -111 },
	{ x: 716, y: -28 },
	{ x: 707, y: 531 },
	{ x: 721, y: 343 },
	{ x: 503, y: 770 },
	{ x: 700, y: -272 },
	{ x: 894, y: 412 },
	{ x: 640, y: 449 },
	{ x: 775, y: 253 },
	{ x: 608, y: 769 },
	{ x: 664, y: -159 },
	{ x: 430, y: 290 },
	{ x: 741, y: 61 },
	{ x: 511, y: 655 },
	{ x: 602, y: -279 },
	{ x: 568, y: 187 },
	{ x: 595, y: -95 },
	{ x: 634, y: 69 },
	{ x: 610, y: 549 },
	{ x: 630, y: 359 },
	{ x: 369, y: -42 },
	{ x: 546, y: 305 },
	{ x: 528, y: 411 },
	{ x: 486, y: 495 },
	{ x: 561, y: -183 },
	{ x: 662, y: 155 },
	{ x: 519, y: 63 },
	{ x: 182, y: 150 },
	{ x: 483, y: -33 },
	{ x: 392, y: 472 },
	{ x: 430, y: 370 },
	{ x: 449, y: -111 },
	{ x: 422, y: -241 },
	{ x: 321, y: 346 },
	{ x: 481, y: 205 },
	{ x: 293, y: 645 },
	{ x: 449, y: 101 },
	{ x: 506, y: -255 },
	{ x: 284, y: 170 },
	{ x: 583, y: -6 },
	{ x: 435, y: 703 },
	{ x: 107, y: 67 },
	{ x: 297, y: 65 },
	{ x: 368, y: 221 },
	{ x: 354, y: 701 },
	{ x: 258, y: 276 },
	{ x: 356, y: -192 },
	{ x: 264, y: -22 },
	{ x: 398, y: 575 },
	{ x: 300, y: 448 },
	{ x: 280, y: 536 },
	{ x: 160, y: 250 },
	{ x: 174, y: 499 },
	{ x: 217, y: 401 },
	{ x: 193, y: 46 },
	{ x: 285, y: -104 },
	{ x: 75, y: 181 },
	{ x: 254, y: -183 },
	{ x: 207, y: 605 },
	{ x: 143, y: 357 },
	{ x: 535, y: 563 },
	{ x: 383, y: 80 },
	{ x: 200, y: 463 },
	{ x: 158, y: -58 },
	{ x: 130, y: 300 }
	 ];

    var width = 1400,
        height = 490,
        numberNodes = 100,
        linkGroup = 0;
        //nodeLinks = [];

    var fill = d3.scale.category20();

    var force = d3.layout.force() //.gravity(0.2).charge(-100)
    .size([width, height])
        .nodes(nodes)
    	.linkDistance(90)
        .charge(-360)
        .on("tick", tick);

    var svg = d3.select("#jumbotron").append("svg")
        .attr('id', 'node-canvas')
        .attr("width", width)
        .attr("height", height)

    //set left value after adding to dom
    resize();

    svg.append("rect")
        .attr("width", width)
        .attr("height", height);

    var nodes = force.nodes(),
        links = force.links(),
        node = svg.selectAll(".node"),
        link = svg.selectAll(".link");

    var cursor = svg.append("circle")
        .attr("r", 30)
        .attr("transform", "translate(-100,-100)")
        .attr("class", "cursor");


    function createLink(index) {

        var node = nodes[index],
        	nodeSelected = svg.select("#id_" + node.index).classed("active linkgroup_"+ linkGroup, true),
        	skip = false,
        	limit = (limit) ? limit : 5;
        

        nodes.forEach(function (target) {
            var selected = svg.select("#id_" + target.index),
            	x = selected.attr('cx') - nodeSelected.attr('cx'),
                y = selected.attr('cy') - nodeSelected.attr('cy');


            if (Math.sqrt(x * x + y * y) < 160) {

                /*fif (nodeLinks[index]) {
                    var nodeLinksLinks = nodeLinks[index];

                    or (var i = 0; i < nodeLinksLinks.length; i++) {
                        if (nodeLinksLinks[i] == target) {
                            skip = true;
                            break;
                        }
                    }
                }*/

                //if (!skip) {
            	var link  = {
                    source: node,
                    target: target
                }

                links.push(link);

                /*if (nodeLinks[index]) {
                    nodeLinks[index].push(target);
                } else {
                    nodeLinks[index] = [target];
                }*/

                //}
            }

        });

        restart();
    }


    function tick() {
		link.attr("x1", function(d) { return d.source.x; })
		    .attr("y1", function(d) { return d.source.y; })
		    .attr("x2", function(d) { return d.target.x; })
		    .attr("y2", function(d) { return d.target.y; });

		node.attr("cx", function(d) { return d.x; })
		    .attr("cy", function(d) { return d.y; });
    }


    function restart() {

        node = node.data(nodes);

        node.enter().insert("circle", ".cursor")
            .attr("class", "node")
            .attr("r", 12)
            .attr("id", function (d, i) {
                return ("id_" + i)
            })
            .call(force.drag);

        link = link.data(links);

        link.enter().insert("line", ".node")
            .attr("class", "link active linkgroup_"+ linkGroup);

        force.start();

        resetLink(linkGroup);
        linkGroup++;
    }

    function resetLink(num){
    	setTimeout(resetColors, 1200, num)
    }
		
    function resetColors(num){
		svg.selectAll(".linkgroup_"+ num).classed('active', false)
    }

    function getRandomInt(min, max) {
        return Math.floor(Math.random() * (max - min + 1)) + min;
    }

    window.onresize = function(){
    	//console.log('resize')
        resize(); 	
    }

    function resize() {
    	var nodeC = document.getElementById('node-canvas');
    		wW = window.innerWidth;

    	nodeC.style.left = ((wW - width) / 2 ) + 'px';    	
    }

    //kick things off
    function init() {
        //console.log('init')

	    restart();

	    setTimeout(createLink, 3200, 47);
	    setTimeout(createLink, 1400, 78);
	    setTimeout(createLink, 2500, 54);
	    setTimeout(createLink, 3600, 50);
	    setTimeout(createLink, 4400, 55);
	    setTimeout(createLink, 4800, 62);
	    setTimeout(createLink, 5200, 65);
	    setTimeout(createLink, 6600, 3);
	    setTimeout(createLink, 7400, 16);
	    setTimeout(createLink, 7400, 12);
	    setTimeout(createLink, 8000, 97);        
    }    

	Serf.Nodes = {};
    Serf.Nodes.init = init;

})();