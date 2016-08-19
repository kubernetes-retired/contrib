PerfDashApp.prototype.buildRangeChanged = function() {
    this.labelChanged();
};

PerfDashApp.prototype.resetBuildRange = function() {    
    this.minBuild = parseInt(Math.min.apply(Math, this.builds));
    this.maxBuild = parseInt(Math.max.apply(Math, this.builds));

    this.buildRangeChanged();
};